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

	Transaction(execute func(st StateDBTransaction))
}

type stateDBImpl struct {
	logger hclog.Logger

	storage Storage

	cached *fastcache.Cache

	txnMux sync.Mutex
}

func NewStateDB(storage Storage, logger hclog.Logger) StateDB {
	return &stateDBImpl{
		logger:  logger.Named("state"),
		storage: storage,
		cached:  fastcache.New(32 * 1024 * 1024),
	}
}

func (db *stateDBImpl) Get(k []byte) ([]byte, bool, error) {
	if db.cached != nil {
		if enc := db.cached.Get(nil, k); enc != nil {
			return enc, true, nil
		}
	}

	v, ok, err := db.storage.Get(k)
	if err != nil {
		db.logger.Error("get", "err", err)
	}

	// write-back cache
	if err == nil && ok && db.cached != nil {
		db.cached.Set(k, v)
	}

	return v, ok, err
}

func (db *stateDBImpl) GetCode(hash types.Hash) ([]byte, bool) {
	perfix := append(codePrefix, hash.Bytes()...)
	if db.cached != nil {
		if enc := db.cached.Get(nil, perfix); enc != nil {
			return enc, true
		}
	}

	v, ok, err := db.storage.Get(perfix)
	if err != nil {
		db.logger.Error("get code", "err", err)
	}

	// write-back cache
	if err == nil && ok && db.cached != nil {
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

	t := &Trie{
		root:    n,
		stateDB: db,
	}

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

func (db *stateDBImpl) Transaction(execute func(StateDBTransaction)) {
	db.txnMux.Lock()
	defer db.txnMux.Unlock()

	// get exclusive transaction reference from pool
	stateDBTxnRef := stateTxnPool.Get().(*stateDBTxn)
	// return exclusive transaction reference to pool
	defer stateTxnPool.Put(stateDBTxnRef)

	stateDBTxnRef.stateDB = db
	stateDBTxnRef.storage = db.storage
	stateDBTxnRef.cancel.Store(false)

	// clean up
	defer stateDBTxnRef.Reset()

	// execute transaction
	execute(stateDBTxnRef)

	// update cache
	for _, pair := range stateDBTxnRef.db {
		db.cached.Set(pair.key, pair.value)
	}
}
