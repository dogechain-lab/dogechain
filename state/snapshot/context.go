// Copyright 2022 The go-ethereum Authors
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
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"time"

	"github.com/dogechain-lab/dogechain/helper/kvdb"
	"github.com/dogechain-lab/dogechain/helper/kvdb/memorydb"
	"github.com/dogechain-lab/dogechain/helper/rawdb"
	"github.com/dogechain-lab/dogechain/types"
)

const (
	snapAccount = "account" // Identifier of account snapshot generation
	snapStorage = "storage" // Identifier of storage snapshot generation
)

// generatorStats is a collection of statistics gathered by the snapshot generator
// for logging purposes.
type generatorStats struct {
	origin   uint64            // Origin prefix where generation started
	start    time.Time         // Timestamp when generation started
	accounts uint64            // Number of accounts indexed(generated or recovered)
	slots    uint64            // Number of storage slots indexed(generated or recovered)
	dangling uint64            // Number of dangling storage slots
	storage  types.StorageSize // Total account and storage slot size(generation or recovery)
	logger   kvdb.Logger       // logger
}

// Log creates an contextual log with the given message and the context pulled
// from the internally maintained statistics.
func (gs *generatorStats) Log(msg string, root types.Hash, marker []byte) {
	var ctx []interface{}
	if root != (types.Hash{}) {
		ctx = append(ctx, []interface{}{"root", root}...)
	}

	// Figure out whether we're after or within an account
	switch len(marker) {
	case types.HashLength:
		ctx = append(ctx, []interface{}{"at", types.BytesToHash(marker)}...)
	case 2 * types.HashLength:
		ctx = append(ctx, []interface{}{
			"in", types.BytesToHash(marker[:types.HashLength]),
			"at", types.BytesToHash(marker[types.HashLength:]),
		}...)
	}

	// Add the usual measurements
	ctx = append(ctx, []interface{}{
		"accounts", gs.accounts,
		"slots", gs.slots,
		"storage", gs.storage,
		"dangling", gs.dangling,
		"elapsed", types.PrettyDuration(time.Since(gs.start)),
	}...)

	// Calculate the estimated indexing time based on current stats
	if len(marker) > 0 {
		if done := binary.BigEndian.Uint64(marker[:8]) - gs.origin; done > 0 {
			left := math.MaxUint64 - binary.BigEndian.Uint64(marker[:8])

			speed := done/uint64(time.Since(gs.start)/time.Millisecond+1) + 1 // +1s to avoid division by zero
			ctx = append(ctx, []interface{}{
				"eta", types.PrettyDuration(time.Duration(left/speed) * time.Millisecond),
			}...)
		}
	}

	gs.logger.Info(msg, ctx...)
}

// generatorContext carries a few global values to be shared by all generation functions.
type generatorContext struct {
	stats           *generatorStats     // Generation statistic collection
	db              kvdb.KVBatchStorage // Key-value store containing the snapshot data
	account         *holdableIterator   // Iterator of account snapshot data
	storage         *holdableIterator   // Iterator of storage snapshot data
	batch           kvdb.Batch          // Database batch for writing batch data atomically
	logged          time.Time           // The timestamp when last generation progress was displayed
	generateMetrics *Metrics            // The metric
}

// newGeneratorContext initializes the context for generation.
func newGeneratorContext(
	generateMetrics *Metrics,
	stats *generatorStats,
	db kvdb.KVBatchStorage,
	accMarker []byte,
	storageMarker []byte,
) *generatorContext {
	ctx := &generatorContext{
		stats:           stats,
		db:              db,
		batch:           db.NewBatch(),
		logged:          time.Now(),
		generateMetrics: generateMetrics,
	}

	ctx.openIterator(snapAccount, accMarker)
	ctx.openIterator(snapStorage, storageMarker)

	return ctx
}

// openIterator constructs global account and storage snapshot iterators
// at the interrupted position. These iterators should be reopened from time
// to time to avoid blocking leveldb compaction for a long time.
func (ctx *generatorContext) openIterator(kind string, start []byte) {
	if kind == snapAccount {
		iter := ctx.db.NewIterator(rawdb.SnapshotAccountPrefix, start)
		ctx.account = newHoldableIterator(
			rawdb.NewKeyLengthIterator(iter, rawdb.SnapshotPrefixLength+types.HashLength),
		)

		return
	}

	iter := ctx.db.NewIterator(rawdb.SnapshotStoragePrefix, start)
	ctx.storage = newHoldableIterator(
		rawdb.NewKeyLengthIterator(iter, rawdb.SnapshotPrefixLength+2*types.HashLength),
	)
}

// reopenIterator releases the specified snapshot iterator and re-open it
// in the next position. It's aimed for not blocking leveldb compaction.
func (ctx *generatorContext) reopenIterator(kind string) {
	// Shift iterator one more step, so that we can reopen
	// the iterator at the right position.
	var iter = ctx.account
	if kind == snapStorage {
		iter = ctx.storage
	}

	hasNext := iter.Next()

	if !hasNext {
		// Iterator exhausted, release forever and create an already exhausted virtual iterator
		iter.Release()

		if kind == snapAccount {
			ctx.account = newHoldableIterator(memorydb.New().NewIterator(nil, nil))

			return
		}

		ctx.storage = newHoldableIterator(memorydb.New().NewIterator(nil, nil))

		return
	}

	next := iter.Key()
	iter.Release()
	ctx.openIterator(kind, next[rawdb.SnapshotPrefixLength:])
}

// close releases all the held resources.
func (ctx *generatorContext) close() {
	ctx.account.Release()
	ctx.storage.Release()
}

// iterator returns the corresponding iterator specified by the kind.
func (ctx *generatorContext) iterator(kind string) *holdableIterator {
	if kind == snapAccount {
		return ctx.account
	}

	return ctx.storage
}

// removeStorageBefore deletes all storage entries which are located before
// the specified account. When the iterator touches the storage entry which
// is located in or outside the given account, it stops and holds the current
// iterated element locally.
func (ctx *generatorContext) removeStorageBefore(account types.Hash) {
	var (
		count uint64
		// start = time.Now()
		iter = ctx.storage
	)

	for iter.Next() {
		key := iter.Key()

		// the key length already set, dont worry about slice out of bound
		if bytes.Compare(
			key[rawdb.SnapshotPrefixLength:rawdb.SnapshotPrefixLength+types.HashLength],
			account.Bytes(),
		) >= 0 {
			iter.Hold()

			break
		}

		count++

		ctx.batch.Delete(key)

		if ctx.batch.ValueSize() > kvdb.IdealBatchSize {
			ctx.batch.Write()
			ctx.batch.Reset()
		}
	}

	// snapStorageCleanCounter.Inc(time.Since(start).Nanoseconds())
	ctx.stats.dangling += count
}

// removeStorageAt deletes all storage entries which are located in the specified
// account. When the iterator touches the storage entry which is outside the given
// account, it stops and holds the current iterated element locally. An error will
// be returned if the initial position of iterator is not in the given account.
func (ctx *generatorContext) removeStorageAt(account types.Hash) error {
	var (
		count int64
		// start = time.Now()
		iter = ctx.storage
	)

	for iter.Next() {
		key := iter.Key()
		// the key length already set, dont worry about slice out of bound
		cmp := bytes.Compare(
			key[rawdb.SnapshotPrefixLength:rawdb.SnapshotPrefixLength+types.HashLength],
			account.Bytes(),
		)

		if cmp < 0 {
			return errors.New("invalid iterator position")
		}

		if cmp > 0 {
			iter.Hold()

			break
		}

		count++

		ctx.batch.Delete(key)

		if ctx.batch.ValueSize() > kvdb.IdealBatchSize {
			ctx.batch.Write()
			ctx.batch.Reset()
		}
	}

	// snapWipedStorageMeter.Mark(count)
	// snapStorageCleanCounter.Inc(time.Since(start).Nanoseconds())

	return nil
}

// removeStorageLeft deletes all storage entries which are located after
// the current iterator position.
func (ctx *generatorContext) removeStorageLeft() {
	var (
		count uint64
		// start = time.Now()
		iter = ctx.storage
	)

	for iter.Next() {
		count++

		ctx.batch.Delete(iter.Key())

		if ctx.batch.ValueSize() > kvdb.IdealBatchSize {
			ctx.batch.Write()
			ctx.batch.Reset()
		}
	}

	// NOTE: collect metrics
	// snapDanglingStorageMeter.Mark(int64(count))
	// snapStorageCleanCounter.Inc(time.Since(start).Nanoseconds())

	ctx.stats.dangling += count
}
