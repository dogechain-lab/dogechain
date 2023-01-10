package itrie

import (
	"errors"
	"fmt"
	"sync"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/dogechain-lab/dogechain/state"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/hashicorp/go-hclog"
	"go.uber.org/atomic"
)

var (
	// codePrefix is the code prefix for leveldb
	codePrefix = []byte("code")

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

func (db *stateDBImpl) GetMetrics() Metrics {
	return db.metrics
}

func (db *stateDBImpl) Logger() hclog.Logger {
	return db.logger
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
	perfix := append(codePrefix, hash.Bytes()...)
	if enc := db.codeCache.Get(nil, perfix); enc != nil {
		db.metrics.codeCacheHitInc()

		return enc, true
	}

	db.metrics.codeCacheMissInc()

	// start observe disk read time
	observe := db.metrics.codeDiskReadSecondsObserve()

	v, ok, err := db.storage.Get(perfix)
	if err != nil {
		db.logger.Error("failed to get code", "err", err)
	}

	// end observe disk read time, if err != nil, observe will be a no-op
	if err == nil {
		observe()
	}

	// write-back cache
	if err == nil && ok {
		db.cached.Set(perfix, v)
	}

	if !ok {
		return []byte{}, false
	}

	return v, true
}

func (db *stateDBImpl) NewSnapshot() state.Snapshot {
	t := NewTrie()
	t.stateDB = db

	return t
}

func (db *stateDBImpl) NewSnapshotAt(root types.Hash) (state.Snapshot, error) {
	if root == types.EmptyRootHash {
		// empty state
		return db.NewSnapshot(), nil
	}

	n, ok, err := GetNode(root.Bytes(), db)

	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, fmt.Errorf("state not found at hash %s", root)
	}

	t := NewTrie()
	t.root = n
	t.stateDB = db

	return t, nil
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
