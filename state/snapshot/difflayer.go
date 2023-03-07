// Copyright 2019 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package snapshot

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dogechain-lab/dogechain/helper/kvdb"
	"github.com/dogechain-lab/dogechain/helper/metrics"
	"github.com/dogechain-lab/dogechain/helper/rlp"
	"github.com/dogechain-lab/dogechain/state/stypes"
	"github.com/dogechain-lab/dogechain/types"
	bloomfilter "github.com/holiman/bloomfilter/v2"
)

var (
	// aggregatorMemoryLimit is the maximum size of the bottom-most diff layer
	// that aggregates the writes from above until it's flushed into the disk
	// layer.
	//
	// Note, bumping this up might drastically increase the size of the bloom
	// filters that's stored in every diff layer. Don't do that without fully
	// understanding all the implications.
	aggregatorMemoryLimit = uint64(4 * 1024 * 1024)

	// aggregatorItemLimit is an approximate number of items that will end up
	// in the agregator layer before it's flushed out to disk. A plain account
	// weighs around 14B (+hash), a storage slot 32B (+hash), a deleted slot
	// 0B (+hash). Slots are mostly set/unset in lockstep, so that average at
	// 16B (+hash). All in all, the average entry seems to be 15+32=47B. Use a
	// smaller number to be on the safe side.
	aggregatorItemLimit = aggregatorMemoryLimit / 42

	// bloomTargetError is the target false positive rate when the aggregator
	// layer is at its fullest. The actual value will probably move around up
	// and down from this number, it's mostly a ballpark figure.
	//
	// Note, dropping this down might drastically increase the size of the bloom
	// filters that's stored in every diff layer. Don't do that without fully
	// understanding all the implications.
	bloomTargetError = 0.02

	// bloomSize is the ideal bloom filter size given the maximum number of items
	// it's expected to hold and the target false positive error rate.
	bloomSize = math.Ceil(float64(aggregatorItemLimit) * math.Log(bloomTargetError) / math.Log(1/math.Pow(2, math.Log(2))))

	// bloomFuncs is the ideal number of bits a single entry should set in the
	// bloom filter to keep its size to a minimum (given it's size and maximum
	// entry count).
	bloomFuncs = math.Round((bloomSize / float64(aggregatorItemLimit)) * math.Log(2))

	// the bloom offsets are runtime constants which determines which part of the
	// account/storage hash the hasher functions looks at, to determine the
	// bloom key for an account/slot. This is randomized at init(), so that the
	// global population of nodes do not all display the exact same behaviour with
	// regards to bloom content
	bloomDestructHasherOffset = 0
	bloomAccountHasherOffset  = 0
	bloomStorageHasherOffset  = 0
)

// diffLayer represents a collection of modifications made to a state snapshot
// after running a block on top. It contains one sorted list for the account trie
// and one-one list for each storage tries.
//
// The goal of a diff layer is to act as a journal, tracking recent modifications
// made to the state, that have not yet graduated into a semi-immutable state.
type diffLayer struct {
	origin *diskLayer // Base disk layer to directly use on bloom misses
	parent snapshot   // Parent snapshot modified by this one, never nil
	memory uint64     // Approximate guess as to how much memory we use

	root  types.Hash // Root hash to which this snapshot diff belongs to
	stale uint32     // Signals that the layer became stale (state progressed)

	// destructSet is a very special helper marker. If an account is marked as
	// deleted, then it's recorded in this set. However it's allowed that an account
	// is included here but still available in other sets(e.g. storageData). The
	// reason is the diff layer includes all the changes in a *block*. It can
	// happen that in the tx_1, account A is self-destructed while in the tx_2
	// it's recreated. But we still need this marker to indicate the "old" A is
	// deleted, all data in other set belongs to the "new" A.
	// Keyed markers for deleted (and potentially) recreated accounts
	destructSet map[types.Hash]struct{}
	// List of account for iteration. If it exists, it's sorted, otherwise it's nil
	accountList []types.Hash
	// Keyed accounts for direct retrieval (nil means deleted)
	accountData map[types.Hash][]byte
	// List of storage slots for iterated retrievals, one per account. Any existing lists are sorted if non-nil
	storageList map[types.Hash][]types.Hash
	// Keyed storage slots for direct retrieval. one per account (nil means deleted)
	storageData map[types.Hash]map[types.Hash][]byte

	// Bloom filter tracking all the diffed items up to the disk layer
	diffed *bloomfilter.Filter

	lock sync.RWMutex

	// shared logger for print out debug info
	logger kvdb.Logger
	// shared snapshot metrics
	snapmetrics *Metrics
}

// newDiffLayer creates a new diff on top of an existing snapshot, whether that's a low
// level persistent database or a hierarchical diff already.
func newDiffLayer(
	parent snapshot,
	root types.Hash,
	destructs map[types.Hash]struct{},
	accounts map[types.Hash][]byte,
	storage map[types.Hash]map[types.Hash][]byte,
	logger kvdb.Logger,
	snapmetrics *Metrics,
) *diffLayer {
	// Create the new layer with some pre-allocated data segments
	dl := &diffLayer{
		parent:      parent,
		root:        root,
		destructSet: destructs,
		accountData: accounts,
		storageData: storage,
		storageList: make(map[types.Hash][]types.Hash),
		logger:      logger,
		snapmetrics: snapmetrics,
	}

	switch parent := parent.(type) {
	case *diskLayer:
		dl.rebloom(parent)
	case *diffLayer:
		dl.rebloom(parent.origin)
	default:
		panic("unknown parent type")
	}

	// Sanity check that accounts or storage slots are never nil
	for accountHash, blob := range accounts {
		if blob == nil {
			panic(fmt.Sprintf("account %s nil", accountHash))
		}

		// Determine memory size and track the dirty writes
		dl.memory += uint64(types.HashLength + len(blob))
		metrics.AddCounter(dl.snapmetrics.dirtyAccountWriteSize, float64(len(blob)))
	}

	for accountHash, slots := range storage {
		if slots == nil {
			panic(fmt.Sprintf("storage %s nil", accountHash))
		}
		// Determine memory size and track the dirty writes
		for _, data := range slots {
			dl.memory += uint64(types.HashLength + len(data))
			metrics.AddCounter(dl.snapmetrics.dirtyStorageWriteSize, float64(len(data)))
		}
	}

	dl.memory += uint64(len(destructs) * types.HashLength)

	return dl
}

// Root returns the root hash for which this snapshot was made.
func (dl *diffLayer) Root() types.Hash {
	return dl.root
}

// Parent returns the subsequent layer of a diff layer.
func (dl *diffLayer) Parent() snapshot {
	dl.lock.RLock()
	defer dl.lock.RUnlock()

	return dl.parent
}

// Stale return whether this layer has become stale (was flattened across) or if
// it's still live.
func (dl *diffLayer) Stale() bool {
	return atomic.LoadUint32(&dl.stale) != 0
}

// Account directly retrieves the account associated with a particular hash in
// the snapshot slim data format.
func (dl *diffLayer) Account(hash types.Hash) (*stypes.Account, error) {
	data, err := dl.AccountRLP(hash)
	if err != nil {
		return nil, err
	}

	if len(data) == 0 { // can be both nil and []byte{}
		return nil, nil
	}

	// slim account
	var slimAccount Account
	if err := rlp.DecodeBytes(data, &slimAccount); err != nil {
		panic(err)
	}

	// copy balance to heap
	balance := new(big.Int).Set(slimAccount.Balance)

	return &stypes.Account{
		Nonce:       slimAccount.Nonce,
		Balance:     balance,
		StorageRoot: types.BytesToHash(slimAccount.Root),
		CodeHash:    slimAccount.CodeHash,
	}, nil
}

// AccountRLP directly retrieves the account RLP associated with a particular
// hash in the snapshot slim data format.
//
// Note the returned account is not a copy, please don't modify it.
func (dl *diffLayer) AccountRLP(hash types.Hash) ([]byte, error) {
	// Check the bloom filter first whether there's even a point in reaching into
	// all the maps in all the layers below
	dl.lock.RLock()

	hit := dl.diffed.Contains(accountBloomHasher(hash))
	if !hit {
		hit = dl.diffed.Contains(destructBloomHasher(hash))
	}

	var origin *diskLayer

	if !hit {
		origin = dl.origin // extract origin while holding the lock
	}

	dl.lock.RUnlock()

	// If the bloom filter misses, don't even bother with traversing the memory
	// diff layers, reach straight into the bottom persistent disk layer
	if origin != nil {
		metrics.CounterInc(dl.snapmetrics.bloomAccountMissCount)

		return origin.AccountRLP(hash)
	}

	// The bloom filter hit, start poking in the internal maps
	return dl.accountRLP(hash, 0)
}

// accountRLP is an internal version of AccountRLP that skips the bloom filter
// checks and uses the internal maps to try and retrieve the data. It's meant
// to be used if a higher layer's bloom filter hit already.
func (dl *diffLayer) accountRLP(hash types.Hash, depth int) ([]byte, error) {
	dl.lock.RLock()
	defer dl.lock.RUnlock()

	// If the layer was flattened into, consider it invalid (any live reference to
	// the original should be marked as unusable).
	if dl.Stale() {
		return nil, ErrSnapshotStale
	}
	// If the account is known locally, return it
	if data, ok := dl.accountData[hash]; ok {
		metrics.CounterInc(dl.snapmetrics.dirtyAccountHitCount)
		metrics.HistogramObserve(dl.snapmetrics.dirtyAccountHitDepth, float64(depth))
		metrics.AddCounter(dl.snapmetrics.dirtyAccountReadSize, float64(len(data)))
		metrics.CounterInc(dl.snapmetrics.bloomAccountTrueHitCount)

		return data, nil
	}
	// If the account is known locally, but deleted, return it
	if _, ok := dl.destructSet[hash]; ok {
		metrics.CounterInc(dl.snapmetrics.dirtyAccountHitCount)
		metrics.HistogramObserve(dl.snapmetrics.dirtyAccountHitDepth, float64(depth))
		metrics.CounterInc(dl.snapmetrics.dirtyAccountInexCount)
		metrics.CounterInc(dl.snapmetrics.bloomAccountTrueHitCount)

		return nil, nil
	}
	// Account unknown to this diff, resolve from parent
	if diff, ok := dl.parent.(*diffLayer); ok {
		return diff.accountRLP(hash, depth+1)
	}

	// Failed to resolve through diff layers, mark a bloom error and use the disk
	metrics.CounterInc(dl.snapmetrics.bloomAccountFalseHitCount)

	return dl.parent.AccountRLP(hash)
}

// Storage directly retrieves the storage data associated with a particular hash,
// within a particular account. If the slot is unknown to this diff, it's parent
// is consulted.
//
// Note the returned slot is not a copy, please don't modify it.
func (dl *diffLayer) Storage(accountHash, storageHash types.Hash) ([]byte, error) {
	// Check the bloom filter first whether there's even a point in reaching into
	// all the maps in all the layers below
	dl.lock.RLock()

	hit := dl.diffed.Contains(storageBloomHasher{accountHash, storageHash})
	if !hit {
		hit = dl.diffed.Contains(destructBloomHasher(accountHash))
	}

	var origin *diskLayer
	if !hit {
		origin = dl.origin // extract origin while holding the lock
	}

	dl.lock.RUnlock()

	// If the bloom filter misses, don't even bother with traversing the memory
	// diff layers, reach straight into the bottom persistent disk layer
	if origin != nil {
		metrics.CounterInc(dl.snapmetrics.bloomStorageMissCount)

		return origin.Storage(accountHash, storageHash)
	}
	// The bloom filter hit, start poking in the internal maps
	return dl.storage(accountHash, storageHash, 0)
}

// storage is an internal version of Storage that skips the bloom filter checks
// and uses the internal maps to try and retrieve the data. It's meant  to be
// used if a higher layer's bloom filter hit already.
func (dl *diffLayer) storage(accountHash, storageHash types.Hash, depth int) ([]byte, error) {
	dl.lock.RLock()
	defer dl.lock.RUnlock()

	// If the layer was flattened into, consider it invalid (any live reference to
	// the original should be marked as unusable).
	if dl.Stale() {
		return nil, ErrSnapshotStale
	}
	// If the account is known locally, try to resolve the slot locally
	if storage, ok := dl.storageData[accountHash]; ok {
		if data, ok := storage[storageHash]; ok {
			metrics.CounterInc(dl.snapmetrics.dirtyStorageHitCount)
			metrics.HistogramObserve(dl.snapmetrics.dirtyStorageHitDepth, float64(depth))
			metrics.CounterInc(dl.snapmetrics.bloomStorageTrueHitCount)

			if n := len(data); n > 0 {
				metrics.AddCounter(dl.snapmetrics.dirtyStorageReadSize, float64(n))
			} else {
				metrics.CounterInc(dl.snapmetrics.dirtyStorageInexCount)
			}

			return data, nil
		}
	}
	// If the account is known locally, but deleted, return an empty slot
	if _, ok := dl.destructSet[accountHash]; ok {
		metrics.CounterInc(dl.snapmetrics.dirtyStorageHitCount)
		metrics.HistogramObserve(dl.snapmetrics.dirtyStorageHitDepth, float64(depth))
		metrics.CounterInc(dl.snapmetrics.dirtyStorageInexCount)
		metrics.CounterInc(dl.snapmetrics.bloomStorageTrueHitCount)

		return nil, nil
	}
	// Storage slot unknown to this diff, resolve from parent
	if diff, ok := dl.parent.(*diffLayer); ok {
		return diff.storage(accountHash, storageHash, depth+1)
	}

	// Failed to resolve through diff layers, mark a bloom error and use the disk
	metrics.CounterInc(dl.snapmetrics.bloomStorageFalseHitCount)

	return dl.parent.Storage(accountHash, storageHash)
}

// Update creates a new layer on top of the existing snapshot diff tree with
// the specified data items.
func (dl *diffLayer) Update(
	blockRoot types.Hash,
	destructs map[types.Hash]struct{},
	accounts map[types.Hash][]byte,
	storage map[types.Hash]map[types.Hash][]byte,
	logger kvdb.Logger,
) *diffLayer {
	return newDiffLayer(dl, blockRoot, destructs, accounts, storage, logger, dl.snapmetrics)
}

// AccountList returns a sorted list of all accounts in this diffLayer, including
// the deleted ones.
//
// Note, the returned slice is not a copy, so do not modify it.
func (dl *diffLayer) AccountList() []types.Hash {
	// If an old list already exists, return it
	dl.lock.RLock()

	list := dl.accountList

	dl.lock.RUnlock()

	if list != nil {
		return list
	}

	// No old sorted account list exists, generate a new one
	dl.lock.Lock()
	defer dl.lock.Unlock()

	dl.accountList = make([]types.Hash, 0, len(dl.destructSet)+len(dl.accountData))

	for hash := range dl.accountData {
		dl.accountList = append(dl.accountList, hash)
	}

	for hash := range dl.destructSet {
		if _, ok := dl.accountData[hash]; !ok {
			dl.accountList = append(dl.accountList, hash)
		}
	}

	sort.Sort(hashes(dl.accountList))

	dl.memory += uint64(len(dl.accountList) * types.HashLength)

	return dl.accountList
}

// StorageList returns a sorted list of all storage slot hashes in this diffLayer
// for the given account. If the whole storage is destructed in this layer, then
// an additional flag *destructed = true* will be returned, otherwise the flag is
// false. Besides, the returned list will include the hash of deleted storage slot.
// Note a special case is an account is deleted in a prior tx but is recreated in
// the following tx with some storage slots set. In this case the returned list is
// not empty but the flag is true.
//
// Note, the returned slice is not a copy, so do not modify it.
func (dl *diffLayer) StorageList(accountHash types.Hash) ([]types.Hash, bool) {
	dl.lock.RLock()

	_, destructed := dl.destructSet[accountHash]

	if _, ok := dl.storageData[accountHash]; !ok {
		// Account not tracked by this layer
		dl.lock.RUnlock()

		return nil, destructed
	}

	// If an old list already exists, return it
	if list, exist := dl.storageList[accountHash]; exist {
		dl.lock.RUnlock()

		return list, destructed // the cached list can't be nil
	}

	dl.lock.RUnlock()

	// No old sorted account list exists, generate a new one
	dl.lock.Lock()
	defer dl.lock.Unlock()

	storageMap := dl.storageData[accountHash]
	storageList := make([]types.Hash, 0, len(storageMap))

	for k := range storageMap {
		storageList = append(storageList, k)
	}

	sort.Sort(hashes(storageList))

	dl.storageList[accountHash] = storageList
	dl.memory += uint64(len(dl.storageList)*types.HashLength + types.HashLength)

	return storageList, destructed
}

// flatten pushes all data from this point downwards, flattening everything into
// a single diff at the bottom. Since usually the lowermost diff is the largest,
// the flattening builds up from there in reverse.
func (dl *diffLayer) flatten() snapshot {
	// If the parent is not diff, we're the first in line, return unmodified
	parent, ok := dl.parent.(*diffLayer)
	if !ok {
		return dl
	}

	// Parent is a diff, flatten it first (note, apart from weird corned cases,
	// flatten will realistically only ever merge 1 layer, so there's no need to
	// be smarter about grouping flattens together).
	parent, _ = parent.flatten().(*diffLayer)

	parent.lock.Lock()
	defer parent.lock.Unlock()

	// Before actually writing all our data to the parent, first ensure that the
	// parent hasn't been 'corrupted' by someone else already flattening into it
	if atomic.SwapUint32(&parent.stale, 1) != 0 {
		// we've flattened into the same parent from two children, boo
		panic("parent diff layer is stale")
	}

	// Overwrite all the updated accounts blindly, merge the sorted list
	for hash := range dl.destructSet {
		parent.destructSet[hash] = struct{}{}
		delete(parent.accountData, hash)
		delete(parent.storageData, hash)
	}

	for hash, data := range dl.accountData {
		parent.accountData[hash] = data
	}

	// Overwrite all the updated storage slots (individually)
	for accountHash, storage := range dl.storageData {
		// If storage didn't exist (or was deleted) in the parent, overwrite blindly
		if _, ok := parent.storageData[accountHash]; !ok {
			parent.storageData[accountHash] = storage

			continue
		}
		// Storage exists in both parent and child, merge the slots
		comboData := parent.storageData[accountHash]

		for storageHash, data := range storage {
			comboData[storageHash] = data
		}
	}

	// Return the combo parent
	return &diffLayer{
		parent:      parent.parent,
		origin:      parent.origin,
		root:        dl.root,
		destructSet: parent.destructSet,
		accountData: parent.accountData,
		storageData: parent.storageData,
		storageList: make(map[types.Hash][]types.Hash),
		diffed:      dl.diffed,
		memory:      parent.memory + dl.memory,
		logger:      dl.logger,
	}
}

// rebloom discards the layer's current bloom and rebuilds it from scratch based
// on the parent's and the local diffs.
func (dl *diffLayer) rebloom(origin *diskLayer) {
	dl.lock.Lock()
	defer dl.lock.Unlock()

	defer func(start time.Time) {
		metrics.HistogramObserve(dl.snapmetrics.bloomIndexNanoseconds, float64(time.Since(start).Nanoseconds()))
	}(time.Now())

	// Inject the new origin that triggered the rebloom
	dl.origin = origin

	// Retrieve the parent bloom or create a fresh empty one
	if parent, ok := dl.parent.(*diffLayer); ok {
		parent.lock.RLock()
		dl.diffed, _ = parent.diffed.Copy()
		parent.lock.RUnlock()
	} else {
		dl.diffed, _ = bloomfilter.New(uint64(bloomSize), uint64(bloomFuncs))
	}

	// Iterate over all the accounts and storage slots and index them
	for hash := range dl.destructSet {
		dl.diffed.Add(destructBloomHasher(hash))
	}

	for hash := range dl.accountData {
		dl.diffed.Add(accountBloomHasher(hash))
	}

	for accountHash, slots := range dl.storageData {
		for storageHash := range slots {
			dl.diffed.Add(storageBloomHasher{accountHash, storageHash})
		}
	}

	// Calculate the current false positive rate and update the error rate meter.
	// This is a bit cheating because subsequent layers will overwrite it, but it
	// should be fine, we're only interested in ballpark figures.
	k := float64(dl.diffed.K())
	n := float64(dl.diffed.N())
	m := float64(dl.diffed.M())

	metrics.SetGauge(dl.snapmetrics.bloomErrorCount, math.Pow(1.0-math.Exp((-k)*(n+0.5)/(m-1)), k))
}

// destructBloomHasher is a wrapper around a types.Hash to satisfy the interface
// API requirements of the bloom library used. It's used to convert a destruct
// event into a 64 bit mini hash.
type destructBloomHasher types.Hash

func (h destructBloomHasher) Write(p []byte) (n int, err error) { panic("not implemented") }
func (h destructBloomHasher) Sum(b []byte) []byte               { panic("not implemented") }
func (h destructBloomHasher) Reset()                            { panic("not implemented") }
func (h destructBloomHasher) BlockSize() int                    { panic("not implemented") }
func (h destructBloomHasher) Size() int                         { return 8 }
func (h destructBloomHasher) Sum64() uint64 {
	return binary.BigEndian.Uint64(h[bloomDestructHasherOffset : bloomDestructHasherOffset+8])
}

// accountBloomHasher is a wrapper around a types.Hash to satisfy the interface
// API requirements of the bloom library used. It's used to convert an account
// hash into a 64 bit mini hash.
type accountBloomHasher types.Hash

func (h accountBloomHasher) Write(p []byte) (n int, err error) { panic("not implemented") }
func (h accountBloomHasher) Sum(b []byte) []byte               { panic("not implemented") }
func (h accountBloomHasher) Reset()                            { panic("not implemented") }
func (h accountBloomHasher) BlockSize() int                    { panic("not implemented") }
func (h accountBloomHasher) Size() int                         { return 8 }
func (h accountBloomHasher) Sum64() uint64 {
	return binary.BigEndian.Uint64(h[bloomAccountHasherOffset : bloomAccountHasherOffset+8])
}

// storageBloomHasher is a wrapper around a [2]types.Hash to satisfy the interface
// API requirements of the bloom library used. It's used to convert an account
// hash into a 64 bit mini hash.
type storageBloomHasher [2]types.Hash

func (h storageBloomHasher) Write(p []byte) (n int, err error) { panic("not implemented") }
func (h storageBloomHasher) Sum(b []byte) []byte               { panic("not implemented") }
func (h storageBloomHasher) Reset()                            { panic("not implemented") }
func (h storageBloomHasher) BlockSize() int                    { panic("not implemented") }
func (h storageBloomHasher) Size() int                         { return 8 }
func (h storageBloomHasher) Sum64() uint64 {
	return binary.BigEndian.Uint64(h[0][bloomStorageHasherOffset:bloomStorageHasherOffset+8]) ^
		binary.BigEndian.Uint64(h[1][bloomStorageHasherOffset:bloomStorageHasherOffset+8])
}