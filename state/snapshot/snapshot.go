package snapshot

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"github.com/dogechain-lab/dogechain/helper/kvdb"
	"github.com/dogechain-lab/dogechain/helper/rlp"
	"github.com/dogechain-lab/dogechain/state/stypes"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/hashicorp/go-hclog"
)

var (
	// ErrSnapshotStale is returned from data accessors if the underlying snapshot
	// layer had been invalidated due to the chain progressing forward far enough
	// to not maintain the layer's original state.
	ErrSnapshotStale = errors.New("snapshot stale")

	// ErrNotCoveredYet is returned from data accessors if the underlying snapshot
	// is being generated currently and the requested data item is not yet in the
	// range of accounts covered.
	ErrNotCoveredYet = errors.New("not covered yet")

	// ErrNotConstructed is returned if the callers want to iterate the snapshot
	// while the generation is not finished yet.
	ErrNotConstructed = errors.New("snapshot is not constructed")

	// errSnapshotCycle is returned if a snapshot is attempted to be inserted
	// that forms a cycle in the snapshot tree.
	errSnapshotCycle = errors.New("snapshot cycle")
)

// Snapshot represents the functionality supported by a snapshot storage layer.
type Snapshot interface {
	// Root returns the root hash for which this snapshot was made.
	Root() types.Hash

	// Account directly retrieves the account associated with a particular hash in
	// the snapshot slim data format.
	Account(hash types.Hash) (*stypes.Account, error)

	// AccountRLP directly retrieves the account RLP associated with a particular
	// hash in the snapshot slim data format.
	AccountRLP(hash types.Hash) ([]byte, error)

	// Storage directly retrieves the storage data associated with a particular hash,
	// within a particular account.
	Storage(accountHash, storageHash types.Hash) ([]byte, error)
}

// snapshot is the internal version of the snapshot data layer that supports some
// additional methods compared to the public API.
type snapshot interface {
	Snapshot

	// Parent returns the subsequent layer of a snapshot, or nil if the base was
	// reached.
	//
	// Note, the method is an internal helper to avoid type switching between the
	// disk and diff layers. There is no locking involved.
	Parent() snapshot

	// Update creates a new layer on top of the existing snapshot diff tree with
	// the specified data items.
	//
	// Note, the maps are retained by the method to avoid copying everything.
	Update(
		blockRoot types.Hash,
		destructs map[types.Hash]struct{},
		accounts map[types.Hash][]byte,
		storage map[types.Hash]map[types.Hash][]byte,
	) *diffLayer

	// Journal commits an entire diff hierarchy to disk into a single journal entry.
	// This is meant to be used during shutdown to persist the snapshot without
	// flattening everything down (bad for reorgs).
	Journal(buffer *bytes.Buffer) (types.Hash, error)

	// Stale return whether this layer has become stale (was flattened across) or
	// if it's still live.
	Stale() bool

	// AccountIterator creates an account iterator over an arbitrary layer.
	AccountIterator(seek types.Hash) AccountIterator

	// StorageIterator creates a storage iterator over an arbitrary layer.
	StorageIterator(account types.Hash, seek types.Hash) (StorageIterator, bool)
}

// Config includes the configurations for snapshots.
type Config struct {
	CacheSize  int  // Megabytes permitted to use for read caches
	Recovery   bool // Indicator that the snapshots is in the recovery mode
	NoBuild    bool // Indicator that the snapshots generation is disallowed
	AsyncBuild bool // The snapshot generation is allowed to be constructed asynchronously
}

// Tree is an Ethereum state snapshot tree. It consists of one persistent base
// layer backed by a key-value store, on top of which arbitrarily many in-memory
// diff layers are topped. The memory diffs can form a tree with branching, but
// the disk layer is singleton and common to all. If a reorg goes deeper than the
// disk layer, everything needs to be deleted.
//
// The goal of a state snapshot is twofold: to allow direct access to account and
// storage data to avoid expensive multi-level trie lookups; and to allow sorted,
// cheap iteration of the account/storage tries for sync aid.
type Tree struct {
	config Config              // Snapshots configurations
	diskdb kvdb.KVBatchStorage // Persistent database to store the snapshot
	// triedb *trie.Database           // In-memory cache to access the trie through
	layers map[types.Hash]snapshot // Collection of all known layers
	lock   sync.RWMutex

	logger hclog.Logger

	// Test hooks
	onFlatten func() // Hook invoked when the bottom most diff layers are flattened
}

// New attempts to load an already existing snapshot from a persistent key-value
// store (with a number of memory layers from a journal), ensuring that the head
// of the snapshot matches the expected one.
//
// If the snapshot is missing or the disk layer is broken, the snapshot will be
// reconstructed using both the existing data and the state trie.
// The repair happens on a background thread.
//
// If the memory layers in the journal do not match the disk layer (e.g. there is
// a gap) or the journal is missing, there are two repair cases:
//
//   - if the 'recovery' parameter is true, memory diff-layers and the disk-layer
//     will all be kept. This case happens when the snapshot is 'ahead' of the
//     state trie.
//   - otherwise, the entire snapshot is considered invalid and will be recreated on
//     a background thread.
func New(
	config Config,
	diskdb kvdb.KVBatchStorage,
	root types.Hash,
	logger hclog.Logger,
) (*Tree, error) {
	// Create a new, empty snapshot tree
	snap := &Tree{
		config: config,
		diskdb: diskdb,
		// triedb: triedb,
		layers: make(map[types.Hash]snapshot),
		logger: logger.Named("snapshot"),
	}

	// Attempt to load a previously persisted snapshot and rebuild one if failed
	head, disabled, err := loadSnapshot(diskdb, root, config.CacheSize, config.Recovery, config.NoBuild)
	if disabled {
		snap.logger.Warn("Snapshot maintenance disabled (syncing)")

		return snap, nil
	}

	// Create the building waiter iff the background generation is allowed
	if !config.NoBuild && !config.AsyncBuild {
		defer snap.waitBuild()
	}

	if err != nil {
		snap.logger.Warn("Failed to load snapshot", "err", err)

		if !config.NoBuild {
			snap.Rebuild(root)

			return snap, nil
		}

		return nil, err // Bail out the error, don't rebuild automatically.
	}

	// Existing snapshot loaded, seed all the layers
	for head != nil {
		snap.layers[head.Root()] = head
		head = head.Parent()
	}

	return snap, nil
}

// waitBuild blocks until the snapshot finishes rebuilding. This method is meant
// to be used by tests to ensure we're testing what we believe we are.
func (t *Tree) waitBuild() {
	// Find the rebuild termination channel
	var done chan struct{}

	t.lock.RLock()
	for _, layer := range t.layers {
		if layer, ok := layer.(*diskLayer); ok {
			done = layer.genPending

			break
		}
	}
	t.lock.RUnlock()

	// Wait until the snapshot is generated
	if done != nil {
		<-done
	}
}

// Disable interrupts any pending snapshot generator, deletes all the snapshot
// layers in memory and marks snapshots disabled globally. In order to resume
// the snapshot functionality, the caller must invoke Rebuild.
func (t *Tree) Disable() {
	// Interrupt any live snapshot layers
	t.lock.Lock()
	defer t.lock.Unlock()

	for _, layer := range t.layers {
		switch layer := layer.(type) {
		case *diskLayer:
			// If the base layer is generating, abort it
			if layer.genAbort != nil {
				abort := make(chan *generatorStats)
				layer.genAbort <- abort
				<-abort
			}
			// Layer should be inactive now, mark it as stale
			layer.lock.Lock()
			layer.stale = true
			layer.lock.Unlock()
		case *diffLayer:
			// If the layer is a simple diff, simply mark as stale
			layer.lock.Lock()
			atomic.StoreUint32(&layer.stale, 1)
			layer.lock.Unlock()
		default:
			panic(fmt.Sprintf("unknown layer type: %T", layer))
		}
	}

	t.layers = map[types.Hash]snapshot{}

	// Delete all snapshot liveness information from the database
	batch := t.diskdb.NewBatch()

	// // TODO: delete snapshot in batch
	// rawdb.WriteSnapshotDisabled(batch)
	// rawdb.DeleteSnapshotRoot(batch)
	// rawdb.DeleteSnapshotJournal(batch)
	// rawdb.DeleteSnapshotGenerator(batch)
	// rawdb.DeleteSnapshotRecoveryNumber(batch)
	// // Note, we don't delete the sync progress

	if err := batch.Write(); err != nil {
		t.logger.Error("Failed to disable snapshots", "err", err)
		os.Exit(1)
	}
}

// Snapshot retrieves a snapshot belonging to the given block root, or nil if no
// snapshot is maintained for that block.
func (t *Tree) Snapshot(blockRoot types.Hash) Snapshot {
	t.lock.RLock()
	defer t.lock.RUnlock()

	return t.layers[blockRoot]
}

// Snapshots returns all visited layers from the topmost layer with specific
// root and traverses downward. The layer amount is limited by the given number.
// If nodisk is set, then disk layer is excluded.
func (t *Tree) Snapshots(root types.Hash, limits int, nodisk bool) []Snapshot {
	t.lock.RLock()
	defer t.lock.RUnlock()

	if limits == 0 {
		return nil
	}

	layer := t.layers[root]
	if layer == nil {
		return nil
	}

	var ret []Snapshot

	for {
		if _, isdisk := layer.(*diskLayer); isdisk && nodisk {
			break
		}

		ret = append(ret, layer)

		limits -= 1
		if limits == 0 {
			break
		}

		parent := layer.Parent()
		if parent == nil {
			break
		}

		layer = parent
	}

	return ret
}

// Update adds a new snapshot into the tree, if that can be linked to an existing
// old parent. It is disallowed to insert a disk layer (the origin of all).
func (t *Tree) Update(
	blockRoot types.Hash,
	parentRoot types.Hash,
	destructs map[types.Hash]struct{},
	accounts map[types.Hash][]byte,
	storage map[types.Hash]map[types.Hash][]byte,
) error {
	// Reject noop updates to avoid self-loops in the snapshot tree. This is a
	// special case that can only happen for Clique networks where empty blocks
	// don't modify the state (0 block subsidy).
	//
	// Although we could silently ignore this internally, it should be the caller's
	// responsibility to avoid even attempting to insert such a snapshot.
	if blockRoot == parentRoot {
		return errSnapshotCycle
	}

	// Generate a new snapshot on top of the parent
	parent := t.Snapshot(parentRoot)
	if parent == nil {
		return fmt.Errorf("parent [%#x] snapshot missing", parentRoot)
	}

	//nolint:forcetypeassert
	snap := parent.(snapshot).Update(blockRoot, destructs, accounts, storage)

	// Save the new snapshot for later
	t.lock.Lock()
	defer t.lock.Unlock()

	t.layers[snap.root] = snap

	return nil
}

// Cap traverses downwards the snapshot tree from a head block hash until the
// number of allowed layers are crossed. All layers beyond the permitted number
// are flattened downwards.
//
// Note, the final diff layer count in general will be one more than the amount
// requested. This happens because the bottom-most diff layer is the accumulator
// which may or may not overflow and cascade to disk. Since this last layer's
// survival is only known *after* capping, we need to omit it from the count if
// we want to ensure that *at least* the requested number of diff layers remain.
func (t *Tree) Cap(root types.Hash, layers int) error {
	// Retrieve the head snapshot to cap from
	snap := t.Snapshot(root)
	if snap == nil {
		return fmt.Errorf("snapshot [%#x] missing", root)
	}

	diff, ok := snap.(*diffLayer)
	if !ok {
		return fmt.Errorf("snapshot [%#x] is disk layer", root)
	}

	// If the generator is still running, use a more aggressive cap
	diff.origin.lock.RLock()
	if diff.origin.genMarker != nil && layers > 8 {
		layers = 8
	}

	diff.origin.lock.RUnlock()

	// Run the internal capping and discard all stale layers
	t.lock.Lock()
	defer t.lock.Unlock()

	// Flattening the bottom-most diff layer requires special casing since there's
	// no child to rewire to the grandparent. In that case we can fake a temporary
	// child for the capping and then remove it.
	if layers == 0 {
		// If full commit was requested, flatten the diffs and merge onto disk
		diff.lock.RLock()

		//nolint:forcetypeassert
		base := diffToDisk(diff.flatten().(*diffLayer))

		diff.lock.RUnlock()

		// Replace the entire snapshot tree with the flat base
		t.layers = map[types.Hash]snapshot{base.root: base}

		return nil
	}

	persisted := t.cap(diff, layers)

	// Remove any layer that is stale or links into a stale layer
	children := make(map[types.Hash][]types.Hash)

	for root, snap := range t.layers {
		if diff, ok := snap.(*diffLayer); ok {
			parent := diff.parent.Root()
			children[parent] = append(children[parent], root)
		}
	}

	var remove func(root types.Hash)
	remove = func(root types.Hash) {
		delete(t.layers, root)

		for _, child := range children[root] {
			remove(child)
		}

		delete(children, root)
	}

	for root, snap := range t.layers {
		if snap.Stale() {
			remove(root)
		}
	}

	// If the disk layer was modified, regenerate all the cumulative blooms
	if persisted != nil {
		var rebloom func(root types.Hash)
		rebloom = func(root types.Hash) {
			if diff, ok := t.layers[root].(*diffLayer); ok {
				diff.rebloom(persisted)
			}

			for _, child := range children[root] {
				rebloom(child)
			}
		}

		rebloom(persisted.root)
	}

	return nil
}

// cap traverses downwards the diff tree until the number of allowed layers are
// crossed. All diffs beyond the permitted number are flattened downwards. If the
// layer limit is reached, memory cap is also enforced (but not before).
//
// The method returns the new disk layer if diffs were persisted into it.
//
// Note, the final diff layer count in general will be one more than the amount
// requested. This happens because the bottom-most diff layer is the accumulator
// which may or may not overflow and cascade to disk. Since this last layer's
// survival is only known *after* capping, we need to omit it from the count if
// we want to ensure that *at least* the requested number of diff layers remain.
func (t *Tree) cap(diff *diffLayer, layers int) *diskLayer {
	// Dive until we run out of layers or reach the persistent database
	for i := 0; i < layers-1; i++ {
		// If we still have diff layers below, continue down
		if parent, ok := diff.parent.(*diffLayer); ok {
			diff = parent
		} else {
			// Diff stack too shallow, return without modifications
			return nil
		}
	}
	// We're out of layers, flatten anything below, stopping if it's the disk or if
	// the memory limit is not yet exceeded.
	switch parent := diff.parent.(type) {
	case *diskLayer:
		return nil

	case *diffLayer:
		// Hold the write lock until the flattened parent is linked correctly.
		// Otherwise, the stale layer may be accessed by external reads in the
		// meantime.
		diff.lock.Lock()
		defer diff.lock.Unlock()

		// Flatten the parent into the grandparent. The flattening internally obtains a
		// write lock on grandparent.
		flattened, _ := parent.flatten().(*diffLayer)

		t.layers[flattened.root] = flattened

		// Invoke the hook if it's registered. Ugly hack.
		if t.onFlatten != nil {
			t.onFlatten()
		}

		diff.parent = flattened

		if flattened.memory < aggregatorMemoryLimit {
			// Accumulator layer is smaller than the limit, so we can abort, unless
			// there's a snapshot being generated currently. In that case, the trie
			// will move from underneath the generator so we **must** merge all the
			// partial data down into the snapshot and restart the generation.
			//nolint:forcetypeassert
			if flattened.parent.(*diskLayer).genAbort == nil {
				return nil
			}
		}
	default:
		panic(fmt.Sprintf("unknown data layer: %T", parent))
	}

	// If the bottom-most layer is larger than our memory cap, persist to disk
	bottom, _ := diff.parent.(*diffLayer)

	bottom.lock.RLock()

	base := diffToDisk(bottom)

	bottom.lock.RUnlock()

	t.layers[base.root] = base
	diff.parent = base

	return base
}

// diffToDisk merges a bottom-most diff into the persistent disk layer underneath
// it. The method will panic if called onto a non-bottom-most diff layer.
//
// The disk layer persistence should be operated in an atomic way. All updates should
// be discarded if the whole transition if not finished.
func diffToDisk(bottom *diffLayer) *diskLayer {
	return nil
}

// Journal commits an entire diff hierarchy to disk into a single journal entry.
// This is meant to be used during shutdown to persist the snapshot without
// flattening everything down (bad for reorgs).
//
// The method returns the root hash of the base layer that needs to be persisted
// to disk as a trie too to allow continuing any pending generation op.
func (t *Tree) Journal(root types.Hash) (types.Hash, error) {
	// Retrieve the head snapshot to journal from var snap snapshot
	snap := t.Snapshot(root)
	if snap == nil {
		return types.Hash{}, fmt.Errorf("snapshot [%#x] missing", root)
	}

	// Run the journaling
	t.lock.Lock()
	defer t.lock.Unlock()

	// Firstly write out the metadata of journal
	journal := new(bytes.Buffer)
	if err := rlp.Encode(journal, journalVersion); err != nil {
		return types.Hash{}, err
	}

	diskroot := t.diskRoot()
	if diskroot == (types.Hash{}) {
		return types.Hash{}, errors.New("invalid disk root")
	}

	// Secondly write out the disk layer root, ensure the
	// diff journal is continuous with disk.
	if err := rlp.Encode(journal, diskroot); err != nil {
		return types.Hash{}, err
	}

	// Finally write out the journal of each layer in reverse order.
	base, err := snap.(snapshot).Journal(journal)
	if err != nil {
		return types.Hash{}, err
	}

	// // TODO: Store the journal into the database and return
	// rawdb.WriteSnapshotJournal(t.diskdb, journal.Bytes())

	return base, nil
}

// Rebuild wipes all available snapshot data from the persistent database and
// discard all caches and diff layers. Afterwards, it starts a new snapshot
// generator with the given root hash.
func (t *Tree) Rebuild(root types.Hash) {
	t.lock.Lock()
	defer t.lock.Unlock()

	// TODO: disable snapshot on disk db
	// // Firstly delete any recovery flag in the database. Because now we are
	// // building a brand new snapshot. Also reenable the snapshot feature.
	// rawdb.DeleteSnapshotRecoveryNumber(t.diskdb)
	// rawdb.DeleteSnapshotDisabled(t.diskdb)

	// Iterate over and mark all layers stale
	for _, layer := range t.layers {
		switch layer := layer.(type) {
		case *diskLayer:
			// If the base layer is generating, abort it and save
			if layer.genAbort != nil {
				abort := make(chan *generatorStats)
				layer.genAbort <- abort
				<-abort
			}
			// Layer should be inactive now, mark it as stale
			layer.lock.Lock()
			layer.stale = true
			layer.lock.Unlock()
		case *diffLayer:
			// If the layer is a simple diff, simply mark as stale
			layer.lock.Lock()
			atomic.StoreUint32(&layer.stale, 1)
			layer.lock.Unlock()
		default:
			panic(fmt.Sprintf("unknown layer type: %T", layer))
		}
	}
	// Start generating a new snapshot from scratch on a background thread. The
	// generator will run a wiper first if there's not one running right now.
	t.logger.Info("Rebuilding state snapshot")

	t.layers = map[types.Hash]snapshot{
		root: generateSnapshot(t.diskdb, t.config.CacheSize, root, t.logger),
	}
}

// AccountIterator creates a new account iterator for the specified root hash and
// seeks to a starting account hash.
func (t *Tree) AccountIterator(root types.Hash, seek types.Hash) (AccountIterator, error) {
	ok, err := t.generating()
	if err != nil {
		return nil, err
	}

	if ok {
		return nil, ErrNotConstructed
	}

	return newFastAccountIterator(t, root, seek)
}

// StorageIterator creates a new storage iterator for the specified root hash and
// account. The iterator will be move to the specific start position.
func (t *Tree) StorageIterator(root types.Hash, account types.Hash, seek types.Hash) (StorageIterator, error) {
	ok, err := t.generating()
	if err != nil {
		return nil, err
	}

	if ok {
		return nil, ErrNotConstructed
	}

	return newFastStorageIterator(t, root, account, seek)
}

// Verify iterates the whole state(all the accounts as well as the corresponding storages)
// with the specific root and compares the re-computed hash with the original one.
func (t *Tree) Verify(root types.Hash) error {
	return nil
}

// disklayer is an internal helper function to return the disk layer.
// The lock of snapTree is assumed to be held already.
func (t *Tree) disklayer() *diskLayer {
	var snap snapshot
	for _, s := range t.layers {
		snap = s

		break
	}

	if snap == nil {
		return nil
	}

	switch layer := snap.(type) {
	case *diskLayer:
		return layer
	case *diffLayer:
		return layer.origin
	default:
		panic(fmt.Sprintf("%T: undefined layer", snap))
	}
}

// diskRoot is a internal helper function to return the disk layer root.
// The lock of snapTree is assumed to be held already.
func (t *Tree) diskRoot() types.Hash {
	disklayer := t.disklayer()
	if disklayer == nil {
		return types.Hash{}
	}

	return disklayer.Root()
}

// generating is an internal helper function which reports whether the snapshot
// is still under the construction.
func (t *Tree) generating() (bool, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	layer := t.disklayer()
	if layer == nil {
		return false, errors.New("disk layer is missing")
	}

	layer.lock.RLock()
	defer layer.lock.RUnlock()

	return layer.genMarker != nil, nil
}

// DiskRoot is a external helper function to return the disk layer root.
func (t *Tree) DiskRoot() types.Hash {
	t.lock.Lock()
	defer t.lock.Unlock()

	return t.diskRoot()
}