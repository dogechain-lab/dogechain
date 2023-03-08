package snapshot

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/dogechain-lab/dogechain/helper/hex"
	"github.com/dogechain-lab/dogechain/helper/kvdb"
	"github.com/dogechain-lab/dogechain/helper/rawdb"
	"github.com/dogechain-lab/dogechain/helper/rlp"
	"github.com/dogechain-lab/dogechain/trie"
	"github.com/dogechain-lab/dogechain/types"
)

var (
	// accountCheckRange is the upper limit of the number of accounts involved in
	// each range check. This is a value estimated based on experience. If this
	// range is too large, the failure rate of range proof will increase. Otherwise,
	// if the range is too small, the efficiency of the state recovery will decrease.
	accountCheckRange = 128

	// storageCheckRange is the upper limit of the number of storage slots involved
	// in each range check. This is a value estimated based on experience. If this
	// range is too large, the failure rate of range proof will increase. Otherwise,
	// if the range is too small, the efficiency of the state recovery will decrease.
	storageCheckRange = 1024

	// errMissingTrie is returned if the target trie is missing while the generation
	// is running. In this case the generation is aborted and wait the new signal.
	errMissingTrie = errors.New("missing trie")
)

// generateSnapshot regenerates a brand new snapshot based on an existing state
// database and head block asynchronously. The snapshot is returned immediately
// and generation is continued in the background until done.
func generateSnapshot(
	diskdb kvdb.KVBatchStorage,
	triedb *trie.Database,
	cache int,
	root types.Hash,
	logger kvdb.Logger,
	snapmetrics *Metrics,
) *diskLayer {
	// Create a new disk layer with an initialized state marker at zero
	var (
		stats     = &generatorStats{start: time.Now(), logger: logger}
		batch     = diskdb.NewBatch()
		genMarker = []byte{} // Initialized but empty!
	)

	// write batch to db and journal
	rawdb.WriteSnapshotRoot(batch, root)
	journalProgress(batch, genMarker, stats, logger)

	if err := batch.Write(); err != nil {
		logger.Error("Failed to write initialized state marker", "err", err)
		os.Exit(1)
	}

	base := &diskLayer{
		diskdb:      diskdb,
		triedb:      triedb,
		root:        root,
		cache:       fastcache.New(cache * 1024 * 1024),
		genMarker:   genMarker,
		genPending:  make(chan struct{}),
		genAbort:    make(chan chan *generatorStats),
		logger:      logger,
		snapmetrics: snapmetrics,
	}

	go base.generate(stats)

	logger.Debug("Start snapshot generation", "root", root)

	return base
}

// journalProgress persists the generator stats into the database to resume later.
func journalProgress(
	db kvdb.KVWriter,
	marker []byte,
	stats *generatorStats,
	logger kvdb.Logger,
) {
	// Write out the generator marker. Note it's a standalone disk layer generator
	// which is not mixed with journal. It's ok if the generator is persisted while
	// journal is not.
	entry := journalGenerator{
		Done:   marker == nil,
		Marker: marker,
	}

	if stats != nil {
		entry.Accounts = stats.accounts
		entry.Slots = stats.slots
		entry.Storage = uint64(stats.storage)
	}

	blob, err := rlp.EncodeToBytes(entry)
	if err != nil {
		panic(err) // Cannot happen, here to catch dev errors
	}

	var logstr string

	switch {
	case marker == nil:
		logstr = "done"
	case bytes.Equal(marker, []byte{}):
		logstr = "empty"
	case len(marker) == types.HashLength:
		logstr = fmt.Sprintf("%#x", marker)
	default:
		logstr = fmt.Sprintf("%#x:%#x", marker[:types.HashLength], marker[types.HashLength:])
	}

	logger.Debug("Journalled generator progress", "progress", logstr)

	rawdb.WriteSnapshotGenerator(db, blob)
}

// proofResult contains the output of range proving which can be used
// for further processing regardless if it is successful or not.
type proofResult struct {
	keys     [][]byte   // The key set of all elements being iterated, even proving is failed
	vals     [][]byte   // The val set of all elements being iterated, even proving is failed
	diskMore bool       // Set when the database has extra snapshot states since last iteration
	trieMore bool       // Set when the trie has extra snapshot states(only meaningful for successful proving)
	proofErr error      // Indicator whether the given state range is valid or not
	tr       *trie.Trie // The trie, in case the trie was resolved by the prover (may be nil)
}

// valid returns the indicator that range proof is successful or not.
func (result *proofResult) valid() bool {
	return result.proofErr == nil
}

// last returns the last verified element key regardless of whether the range proof is
// successful or not. Nil is returned if nothing involved in the proving.
func (result *proofResult) last() []byte {
	var last []byte

	if len(result.keys) > 0 {
		last = result.keys[len(result.keys)-1]
	}

	return last
}

// forEach iterates all the visited elements and applies the given callback on them.
// The iteration is aborted if the callback returns non-nil error.
func (result *proofResult) forEach(callback func(key []byte, val []byte) error) error {
	for i := 0; i < len(result.keys); i++ {
		key, val := result.keys[i], result.vals[i]

		if err := callback(key, val); err != nil {
			return err
		}
	}

	return nil
}

// generateAccounts generates the missing snapshot accounts as well as their
// storage slots in the main trie. It's supposed to restart the generation
// from the given origin position.
func generateAccounts(ctx *generatorContext, dl *diskLayer, accMarker []byte, snapmetrics *Metrics) error {
	onAccount := func(key []byte, val []byte, write bool, needDelete bool) error {
		// Make sure to clear all dangling storages before this account
		account := types.BytesToHash(key)
		ctx.removeStorageBefore(account)

		// starting timestamp
		start := time.Now()

		// delete acount and return
		if needDelete {
			rawdb.DeleteAccountSnapshot(ctx.batch, account)
			ctx.metricContext.wipedAccount.Add(1)
			ctx.metricContext.accountWrite.Add(time.Since(start))
			ctx.removeStorageAt(account)

			return nil
		}
		// Retrieve the current account and flatten it into the internal format
		var acc struct {
			Nonce    uint64
			Balance  *big.Int
			Root     types.Hash
			CodeHash []byte
		}

		if err := rlp.DecodeBytes(val, &acc); err != nil {
			dl.logger.Error("Invalid account encountered during snapshot creation", "err", err)
			os.Exit(1)
		}
		// If the account is not yet in-progress, write it out
		if accMarker == nil || !bytes.Equal(account[:], accMarker) {
			dataLen := len(val) // Approximate size, saves us a round of RLP-encoding

			if !write {
				if bytes.Equal(acc.CodeHash, types.EmptyCodeHash.Bytes()) {
					dataLen -= 32
				}

				if acc.Root == types.EmptyRootHash {
					dataLen -= 32
				}

				ctx.metricContext.recoveredAccount.Add(1)
			} else {
				data := SlimAccountRLP(acc.Nonce, acc.Balance, acc.Root, acc.CodeHash)
				dataLen = len(data)
				rawdb.WriteAccountSnapshot(ctx.batch, account, data)
				ctx.metricContext.generatedAccount.Add(1)
			}

			ctx.stats.storage += types.StorageSize(rawdb.SnapshotPrefixLength + types.HashLength + dataLen)
			ctx.stats.accounts++
		}

		// If the snap generation goes here after interrupted, genMarker may go backward
		// when last genMarker is consisted of accountHash and storageHash
		marker := account[:]
		if accMarker != nil && bytes.Equal(marker, accMarker) && len(dl.genMarker) > types.HashLength {
			marker = dl.genMarker[:]
		}

		// If we've exceeded our batch allowance or termination was requested, flush to disk
		if err := dl.checkAndFlush(ctx, marker); err != nil {
			return err
		}

		ctx.metricContext.accountWrite.Add(time.Since(start))

		// If the iterated account is the contract, create a further loop to
		// verify or regenerate the contract storage.
		if acc.Root == types.EmptyRootHash {
			ctx.removeStorageAt(account)
		} else {
			var storeMarker []byte

			if accMarker != nil && bytes.Equal(account[:], accMarker) && len(dl.genMarker) > types.HashLength {
				storeMarker = dl.genMarker[types.HashLength:]
			}

			if err := generateStorages(ctx, dl, dl.root, account, acc.Root, storeMarker); err != nil {
				return err
			}
		}
		// Some account processed, unmark the marker
		accMarker = nil

		return nil
	}

	// Always reset the initial account range as 1 whenever recover from the
	// interruption. TODO(rjl493456442) can we remove it?
	var accountRange = accountCheckRange
	if len(accMarker) > 0 {
		accountRange = 1
	}

	origin := types.CopyBytes(accMarker)

	for {
		id := trie.StateTrieID(dl.root)

		exhausted, last, err := dl.generateRange(ctx, id, rawdb.SnapshotAccountPrefix, snapAccount,
			origin, accountRange, onAccount, FullAccountRLP)
		if err != nil {
			return err // The procedure it aborted, either by external signal or internal error.
		}

		origin = increaseKey(last)

		// Last step, cleanup the storages after the last account.
		// All the left storages should be treated as dangling.
		if origin == nil || exhausted {
			ctx.removeStorageLeft()

			break
		}

		accountRange = accountCheckRange
	}

	return nil
}

// generate is a background thread that iterates over the state and storage tries,
// constructing the state snapshot. All the arguments are purely for statistics
// gathering and logging, since the method surfs the blocks as they arrive, often
// being restarted.
func (dl *diskLayer) generate(stats *generatorStats) {
	var (
		accMarker []byte
		abort     chan *generatorStats
	)

	if len(dl.genMarker) > 0 { // []byte{} is the start, use nil for that
		accMarker = dl.genMarker[:types.HashLength]
	}

	stats.Log("Resuming state snapshot generation", dl.root, dl.genMarker)

	// Initialize the global generator context. The snapshot iterators are
	// opened at the interrupted position because the assumption is held
	// that all the snapshot data are generated correctly before the marker.
	// Even if the snapshot data is updated during the interruption (before
	// or at the marker), the assumption is still held.
	// For the account or storage slot at the interruption, they will be
	// processed twice by the generator(they are already processed in the
	// last run) but it's fine.
	metricContext := dl.snapmetrics.Context()
	ctx := newGeneratorContext(metricContext, stats, dl.diskdb, accMarker, dl.genMarker)

	defer func() {
		dl.snapmetrics.Summary(metricContext)
		ctx.close()
	}()

	// start collecting metrics
	metricContext.Start()

	if err := generateAccounts(ctx, dl, accMarker, dl.snapmetrics); err != nil {
		// Extract the received interruption signal if exists
		var aerr = new(abortError)
		if errors.As(err, &aerr) {
			abort = aerr.abort
		}
		// Aborted by internal error, wait the signal
		if abort == nil {
			abort = <-dl.genAbort
		}

		abort <- stats

		return
	}
	// Snapshot fully generated, set the marker to nil.
	// Note even there is nothing to commit, persist the
	// generator anyway to mark the snapshot is complete.
	journalProgress(ctx.batch, nil, stats, dl.logger)

	if err := ctx.batch.Write(); err != nil {
		dl.logger.Error("Failed to flush batch", "err", err)

		abort = <-dl.genAbort
		abort <- stats

		return
	}

	ctx.batch.Reset()

	dl.logger.Info("Generated state snapshot",
		"accounts", stats.accounts,
		"slots", stats.slots,
		"storage", stats.storage,
		"dangling", stats.dangling,
		"elapsed", types.PrettyDuration(time.Since(stats.start)),
	)

	dl.lock.Lock()
	dl.genMarker = nil
	close(dl.genPending)
	dl.lock.Unlock()

	// Someone will be looking for us, wait it out
	abort = <-dl.genAbort
	abort <- nil
}

// proveRange proves the snapshot segment with particular prefix is "valid".
// The iteration start point will be assigned if the iterator is restored from
// the last interruption. Max will be assigned in order to limit the maximum
// amount of data involved in each iteration.
//
// The proof result will be returned if the range proving is finished, otherwise
// the error will be returned to abort the entire procedure.
func (dl *diskLayer) proveRange(
	ctx *generatorContext,
	trieID *trie.ID,
	prefix []byte,
	kind string,
	origin []byte,
	max int,
	valueConvertFn func([]byte) ([]byte, error),
) (*proofResult, error) {
	var (
		keys     [][]byte
		vals     [][]byte
		proof    = rawdb.NewMemoryDatabase()
		diskMore = false
		iter     = ctx.iterator(kind)
		min      = append(prefix, origin...)
		start    = time.Now()
	)

	for iter.Next() {
		// Ensure the iterated item is always equal or larger than the given origin.
		key := iter.Key()
		if bytes.Compare(key, min) < 0 {
			return nil, errors.New("invalid iteration position")
		}
		// Ensure the iterated item still fall in the specified prefix. If
		// not which means the items in the specified area are all visited.
		// Move the iterator a step back since we iterate one extra element
		// out.
		if !bytes.Equal(key[:len(prefix)], prefix) {
			iter.Hold()

			break
		}
		// Break if we've reached the max size, and signal that we're not
		// done yet. Move the iterator a step back since we iterate one
		// extra element out.
		if len(keys) == max {
			iter.Hold()

			diskMore = true

			break
		}

		keys = append(keys, types.CopyBytes(key[len(prefix):]))

		if valueConvertFn == nil {
			vals = append(vals, types.CopyBytes(iter.Value()))
		} else {
			val, err := valueConvertFn(iter.Value())
			if err != nil {
				// Special case, the state data is corrupted (invalid slim-format account),
				// don't abort the entire procedure directly. Instead, let the fallback
				// generation to heal the invalid data.
				//
				// Here append the original value to ensure that the number of key and
				// value are aligned.
				vals = append(vals, types.CopyBytes(iter.Value()))
				dl.logger.Error("Failed to convert account state data",
					"err", err,
					"kind", kind,
					"prefix", hex.EncodeToHex(prefix),
				)
			} else {
				vals = append(vals, val)
			}
		}
	}

	// Update metrics for database iteration and merkle proving
	if kind == snapStorage {
		ctx.metricContext.storageSnapRead.Add(time.Since(start))
	} else {
		ctx.metricContext.accountSnapRead.Add(time.Since(start))
	}

	defer func(start time.Time) {
		if kind == snapStorage {
			ctx.metricContext.storageProve.Add(time.Since(start))
		} else {
			ctx.metricContext.accountProve.Add(time.Since(start))
		}
	}(time.Now())

	// The snap state is exhausted, pass the entire key/val set for verification
	root := trieID.Root

	if origin == nil && !diskMore {
		stackTr := trie.NewStackTrie(nil)
		for i, key := range keys {
			stackTr.TryUpdate(key, vals[i])
		}

		if gotRoot := stackTr.Hash(); gotRoot != root {
			return &proofResult{
				keys:     keys,
				vals:     vals,
				proofErr: fmt.Errorf("wrong root: have %s want %s", gotRoot, root),
			}, nil
		}

		return &proofResult{keys: keys, vals: vals}, nil
	}
	// Snap state is chunked, generate edge proofs for verification.
	tr, err := trie.New(trieID, dl.triedb, dl.logger)
	if err != nil {
		ctx.stats.Log("Trie missing, state snapshotting paused", dl.root, dl.genMarker)

		return nil, errMissingTrie
	}
	// Firstly find out the key of last iterated element.
	var last []byte
	if len(keys) > 0 {
		last = keys[len(keys)-1]
	}
	// Generate the Merkle proofs for the first and last element
	if origin == nil {
		origin = types.Hash{}.Bytes()
	}

	if err := tr.Prove(origin, 0, proof); err != nil {
		dl.logger.Debug("Failed to prove range", "kind", kind, "origin", origin, "err", err)

		return &proofResult{
			keys:     keys,
			vals:     vals,
			diskMore: diskMore,
			proofErr: err,
			tr:       tr,
		}, nil
	}

	if last != nil {
		if err := tr.Prove(last, 0, proof); err != nil {
			dl.logger.Debug("Failed to prove range", "kind", kind, "last", last, "err", err)

			return &proofResult{
				keys:     keys,
				vals:     vals,
				diskMore: diskMore,
				proofErr: err,
				tr:       tr,
			}, nil
		}
	}
	// Verify the snapshot segment with range prover, ensure that all flat states
	// in this range correspond to merkle trie.
	cont, err := trie.VerifyRangeProof(root, origin, last, keys, vals, proof)

	return &proofResult{
			keys:     keys,
			vals:     vals,
			diskMore: diskMore,
			trieMore: cont,
			proofErr: err,
			tr:       tr},
		nil
}

// onStateCallback is a function that is called by generateRange, when processing a range of
// accounts or storage slots. For each element, the callback is invoked.
//
// - If 'delete' is true, then this element (and potential slots) needs to be deleted from the snapshot.
// - If 'write' is true, then this element needs to be updated with the 'val'.
// - If 'write' is false, then this element is already correct, and needs no update.
// The 'val' is the canonical encoding of the value (not the slim format for accounts)
//
// However, for accounts, the storage trie of the account needs to be checked. Also,
// dangling storages(storage exists but the corresponding account is missing) need to
// be cleaned up.
type onStateCallback func(key []byte, val []byte, write bool, needDelete bool) error

// generateRange generates the state segment with particular prefix. Generation can
// either verify the correctness of existing state through range-proof and skip
// generation, or iterate trie to regenerate state on demand.
func (dl *diskLayer) generateRange(
	ctx *generatorContext,
	trieID *trie.ID,
	prefix []byte,
	kind string,
	origin []byte,
	max int,
	onState onStateCallback,
	valueConvertFn func([]byte) ([]byte, error),
) (bool, []byte, error) {
	// Use range prover to check the validity of the flat state in the range
	result, err := dl.proveRange(ctx, trieID, prefix, kind, origin, max, valueConvertFn)
	if err != nil {
		return false, nil, err
	}

	last := result.last()

	// The range prover says the range is correct, skip trie iteration
	if result.valid() {
		ctx.metricContext.successfulRangeProof.Add(1)

		// The verification is passed, process each state with the given
		// callback function. If this state represents a contract, the
		// corresponding storage check will be performed in the callback
		if err := result.forEach(func(key []byte, val []byte) error { return onState(key, val, false, false) }); err != nil {
			return false, nil, err
		}

		// Only abort the iteration when both database and trie are exhausted
		return !result.diskMore && !result.trieMore, last, nil
	}

	// ctx.stats.logger.Debug("Detected outdated state range",
	// 	"kind", kind,
	// 	"prefix", hex.EncodeToHex(prefix),
	// 	"last", hex.EncodeToHex(last),
	// 	"err", result.proofErr,
	// )
	ctx.metricContext.failedRangeProof.Add(1)

	// Special case, the entire trie is missing. In the original trie scheme,
	// all the duplicated subtries will be filtered out (only one copy of data
	// will be stored). While in the snapshot model, all the storage tries
	// belong to different contracts will be kept even they are duplicated.
	// Track it to a certain extent remove the noise data used for statistics.
	if origin == nil && last == nil {
		if kind == snapStorage {
			ctx.metricContext.missallStorage.Add(1)
		} else {
			ctx.metricContext.missallAccount.Add(1)
		}
	}

	// We use the snap data to build up a cache which can be used by the
	// main account trie as a primary lookup when resolving hashes
	var snapNodeCache kvdb.Database

	if len(result.keys) > 0 {
		snapNodeCache = rawdb.NewMemoryDatabase()
		snapTrieDB := trie.NewDatabase(snapNodeCache, dl.logger)
		snapTrie := trie.NewEmpty(snapTrieDB)

		for i, key := range result.keys {
			snapTrie.Update(key, result.vals[i])
		}

		root, nodes, _ := snapTrie.Commit(false)
		if nodes != nil {
			snapTrieDB.Update(trie.NewWithNodeSet(nodes))
		}

		snapTrieDB.Commit(root, false, nil)
	}

	// Construct the trie for state iteration, reuse the trie
	// if it's already opened with some nodes resolved.
	tr := result.tr
	if tr == nil {
		tr, err = trie.New(trieID, dl.triedb, dl.logger)
		if err != nil {
			ctx.stats.Log("Trie missing, state snapshotting paused", dl.root, dl.genMarker)

			return false, nil, errMissingTrie
		}
	}

	var (
		trieMore       bool
		nodeIt         = tr.NodeIterator(origin)
		iter           = trie.NewIterator(nodeIt)
		kvkeys, kvvals = result.keys, result.vals

		// counters
		count     = 0 // number of states delivered by iterator
		created   = 0 // states created from the trie
		updated   = 0 // states updated from the trie
		deleted   = 0 // states not in trie, but were in snapshot
		untouched = 0 // states already correct

		// timers
		start    = time.Now()
		internal time.Duration
	)

	nodeIt.AddResolver(snapNodeCache)

	for iter.Next() {
		if last != nil && bytes.Compare(iter.Key, last) > 0 {
			trieMore = true

			break
		}

		count++
		created++

		write := true

		for len(kvkeys) > 0 {
			if cmp := bytes.Compare(kvkeys[0], iter.Key); cmp < 0 {
				istart := time.Now()
				// delete the key
				if err := onState(kvkeys[0], nil, false, true); err != nil {
					return false, nil, err
				}

				kvkeys = kvkeys[1:]
				kvvals = kvvals[1:]
				deleted++
				// calculate internal duration
				internal += time.Since(istart)

				continue
			} else if cmp == 0 {
				// the snapshot key can be overwritten
				created--

				if write = !bytes.Equal(kvvals[0], iter.Value); write {
					updated++
				} else {
					untouched++
				}

				kvkeys = kvkeys[1:]
				kvvals = kvvals[1:]
			}

			break
		}

		istart := time.Now()
		// onstate callback
		if err := onState(iter.Key, iter.Value, write, false); err != nil {
			return false, nil, err
		}
		// calculate internal duration
		internal += time.Since(istart)
	}

	if iter.Err != nil {
		return false, nil, iter.Err
	}

	istart := time.Now()
	// Delete all stale snapshot states remaining
	for _, key := range kvkeys {
		if err := onState(key, nil, false, true); err != nil {
			return false, nil, err
		}

		deleted += 1
	}
	// calculate internal duration
	internal += time.Since(istart)

	if kind == snapStorage {
		ctx.metricContext.storageTrieRead.Add(time.Since(start) - internal)
	} else {
		ctx.metricContext.accountTrieRead.Add(time.Since(start) - internal)
	}

	dl.logger.Debug("Regenerated state range",
		"kind", kind,
		"prefix", hex.EncodeToHex(prefix),
		"origin", hex.EncodeToHex(origin),
		"root", trieID.Root,
		"last", hex.EncodeToHex(last),
		"count", count,
		"created", created,
		"updated", updated,
		"untouched", untouched,
		"deleted", deleted,
	)

	// If there are either more trie items, or there are more snap items
	// (in the next segment), then we need to keep working
	return !trieMore && !result.diskMore, last, nil
}

// checkAndFlush checks if an interruption signal is received or the
// batch size has exceeded the allowance.
func (dl *diskLayer) checkAndFlush(ctx *generatorContext, current []byte) error {
	var abort chan *generatorStats

	select {
	case abort = <-dl.genAbort:
	default:
	}

	if ctx.batch.ValueSize() > kvdb.IdealBatchSize || abort != nil {
		if bytes.Compare(current, dl.genMarker) < 0 {
			dl.logger.Error("Snapshot generator went backwards",
				"current", fmt.Sprintf("%x", current),
				"genMarker", fmt.Sprintf("%x", dl.genMarker),
			)
		}

		// Flush out the batch anyway no matter it's empty or not.
		// It's possible that all the states are recovered and the
		// generation indeed makes progress.
		journalProgress(ctx.batch, current, ctx.stats, dl.logger)

		if err := ctx.batch.Write(); err != nil {
			return err
		}

		ctx.batch.Reset()

		dl.lock.Lock()
		dl.genMarker = current
		dl.lock.Unlock()

		if abort != nil {
			ctx.stats.Log("Aborting state snapshot generation", dl.root, current)

			return newAbortError(abort) // bubble up an error for interruption
		}
		// Don't hold the iterators too long, release them to let compactor works
		ctx.reopenIterator(snapAccount)
		ctx.reopenIterator(snapStorage)
	}

	if time.Since(ctx.logged) > 8*time.Second {
		ctx.stats.Log("Generating state snapshot", dl.root, current)
		ctx.logged = time.Now()
	}

	return nil
}

// generateStorages generates the missing storage slots of the specific contract.
// It's supposed to restart the generation from the given origin position.
func generateStorages(
	ctx *generatorContext,
	dl *diskLayer,
	stateRoot types.Hash,
	account types.Hash,
	storageRoot types.Hash,
	storeMarker []byte,
) error {
	onStorage := func(key []byte, val []byte, write bool, needDelete bool) error {
		defer func(start time.Time) {
			ctx.metricContext.storageWrite.Add(time.Since(start))
		}(time.Now())

		if needDelete {
			rawdb.DeleteStorageSnapshot(ctx.batch, account, types.BytesToHash(key))
			ctx.metricContext.wipedStorage.Add(1)

			return nil
		}

		if write {
			rawdb.WriteStorageSnapshot(ctx.batch, account, types.BytesToHash(key), val)
			ctx.metricContext.generatedStorage.Add(1)
		} else {
			ctx.metricContext.recoveredStorage.Add(1)
		}

		ctx.stats.storage += types.StorageSize(rawdb.SnapshotPrefixLength + 2*types.HashLength + len(val))
		ctx.stats.slots++

		// If we've exceeded our batch allowance or termination was requested, flush to disk
		if err := dl.checkAndFlush(ctx, append(account[:], key...)); err != nil {
			return err
		}

		return nil
	}

	// Loop for re-generating the missing storage slots.
	var origin = types.CopyBytes(storeMarker)

	for {
		id := trie.StorageTrieID(stateRoot, account, storageRoot)

		exhausted, last, err := dl.generateRange(
			ctx,
			id,
			append(rawdb.SnapshotStoragePrefix, account.Bytes()...),
			snapStorage,
			origin,
			storageCheckRange,
			onStorage,
			nil,
		)
		if err != nil {
			return err // The procedure it aborted, either by external signal or internal error.
		}

		// Abort the procedure if the entire contract storage is generated
		if exhausted {
			break
		}

		if origin = increaseKey(last); origin == nil {
			break // special case, the last is 0xffffffff...fff
		}
	}

	return nil
}

// increaseKey increase the input key by one bit. Return nil if the entire
// addition operation overflows.
func increaseKey(key []byte) []byte {
	for i := len(key) - 1; i >= 0; i-- {
		key[i]++

		if key[i] != 0x0 {
			return key
		}
	}

	return nil
}

// abortError wraps an interruption signal received to represent the
// generation is aborted by external processes.
type abortError struct {
	abort chan *generatorStats
}

func newAbortError(abort chan *generatorStats) error {
	return &abortError{abort: abort}
}

func (err *abortError) Error() string {
	return "aborted"
}
