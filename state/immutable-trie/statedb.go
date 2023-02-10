package itrie

import (
	"errors"
	"fmt"
	"sync"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/dogechain-lab/dogechain/helper/rawdb"
	"github.com/dogechain-lab/dogechain/state"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/hashicorp/go-hclog"
	"go.uber.org/atomic"
)

var (
	ErrStateTransactionIsCancel = errors.New("transaction is cancel")
)

type StateDBReader interface {
	StorageReader

	GetCode(hash types.Hash) ([]byte, bool)

	NewSnapshot() state.Snapshot
	NewSnapshotAt(types.Hash) (state.Snapshot, error)
}

type StateDB interface {
	StateDBReader

	Transaction(execute func(st StateDBTransaction) error) error

	GetMetrics() Metrics

	Logger() hclog.Logger
}

type stateDBImpl struct {
	logger  hclog.Logger
	metrics Metrics

	storage   Storage
	cached    *fastcache.Cache
	codeCache *fastcache.Cache

	txnMux sync.Mutex
}

func NewStateDB(storage Storage, logger hclog.Logger, metrics Metrics) StateDB {
	return &stateDBImpl{
		logger:    logger.Named("state"),
		storage:   storage,
		cached:    fastcache.New(32 * 1024 * 1024),
		codeCache: fastcache.New(16 * 1024 * 1024),
		metrics:   newDummyMetrics(metrics),
	}
}

func (db *stateDBImpl) newTrie() *Trie {
	return NewTrie()
}

func (db *stateDBImpl) newTrieAt(root types.Hash) (*Trie, error) {
	if root == types.EmptyRootHash {
		// empty state
		return db.newTrie(), nil
	}

	n, ok, err := GetNode(root.Bytes(), db)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage root %s: %w", root, err)
	} else if !ok {
		return nil, fmt.Errorf("state not found at hash %s", root)
	}

	t := db.newTrie()
	t.root = n

	return t, nil
}

func (db *stateDBImpl) GetMetrics() Metrics {
	return db.metrics
}

func (db *stateDBImpl) Logger() hclog.Logger {
	return db.logger
}

func (db *stateDBImpl) Has(p []byte) (bool, error) {
	if db.cached.Has(p) {
		return true, nil
	}

	return db.storage.Has(p)
}

func (db *stateDBImpl) Get(k []byte) ([]byte, bool, error) {
	if enc := db.cached.Get(nil, k); enc != nil {
		db.metrics.accountCacheHitInc()

		return enc, true, nil
	}

	db.metrics.accountCacheMissInc()

	// start observe disk read time
	observe := db.metrics.accountDiskReadSecondsObserve()

	v, ok, err := db.storage.Get(k)
	if err != nil {
		db.logger.Error("failed to get key", "err", err)
	}

	// end observe disk read time, if err != nil, observe will be a no-op
	if err == nil {
		db.metrics.accountReadCountInc()
		observe()
	}

	// write-back cache
	if err == nil && ok {
		db.cached.Set(k, v)
	}

	return v, ok, err
}

func (db *stateDBImpl) GetCode(hash types.Hash) ([]byte, bool) {
	key := rawdb.CodeKey(hash)
	if enc := db.codeCache.Get(nil, key); enc != nil {
		db.metrics.codeCacheHitInc()

		return enc, true
	}

	db.metrics.codeCacheMissInc()

	// start observe disk read time
	observe := db.metrics.codeDiskReadSecondsObserve()

	v, ok, err := db.storage.Get(key)
	if err != nil {
		db.logger.Error("failed to get code", "err", err)
	}

	// end observe disk read time, if err != nil, observe will be a no-op
	if err == nil {
		observe()
	}

	// write-back cache
	if err == nil && ok {
		db.cached.Set(key, v)
	}

	if !ok {
		return []byte{}, false
	}

	return v, true
}

func (db *stateDBImpl) NewSnapshot() state.Snapshot {
	return &Snapshot{state: db, trie: db.newTrie()}
}

func (db *stateDBImpl) NewSnapshotAt(root types.Hash) (state.Snapshot, error) {
	t, err := db.newTrieAt(root)
	if err != nil {
		return nil, err
	}

	return &Snapshot{state: db, trie: t}, nil
}

var stateTxnPool = sync.Pool{
	New: func() interface{} {
		return &stateDBTxn{
			db:     make(map[txnKey]*txnPair),
			cancel: atomic.NewBool(false),
		}
	},
}

func (db *stateDBImpl) Transaction(execute func(StateDBTransaction) error) error {
	db.txnMux.Lock()
	defer db.txnMux.Unlock()

	// get exclusive transaction reference from pool
	stateDBTxnRef, ok := stateTxnPool.Get().(*stateDBTxn)
	if !ok {
		return errors.New("invalid type assertion")
	}

	// return exclusive transaction reference to pool
	defer stateTxnPool.Put(stateDBTxnRef)

	// regardless of whether the user invokes Rollback(), clear transaction again
	stateDBTxnRef.clear()
	// set stateDB, storage and cancel flag
	stateDBTxnRef.stateDB = db
	stateDBTxnRef.storage = db.storage
	stateDBTxnRef.cancel.Store(false)

	observer := db.metrics.stateCommitSecondsObserve()

	// execute transaction
	err := execute(stateDBTxnRef)

	// update cache
	if err == nil {
		// end observe disk write time, if err != nil, observe will be a no-op
		observer()

		for _, pair := range stateDBTxnRef.db {
			if pair.isCode {
				db.codeCache.Set(pair.key, pair.value)
			} else {
				db.cached.Set(pair.key, pair.value)
			}
		}
	}

	return err
}
