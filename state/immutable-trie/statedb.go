package itrie

import (
	"errors"
	"fmt"
	"sync"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/dogechain-lab/dogechain/helper/hex"
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

type StateDBTransaction interface {
	Commit() error
	Rollback()
}

type StateDB interface {
	StorageReader
	StorageWriter

	SetCode(hash types.Hash, code []byte) error
	GetCode(hash types.Hash) ([]byte, bool)

	NewSnapshot() state.Snapshot
	NewSnapshotAt(types.Hash) (state.Snapshot, error)

	ExclusiveTransaction(execute func(st StateDBTransaction))
}

type txnKey string
type txnPair struct {
	key   []byte
	value []byte
}

type stateExTxn struct {
	db    map[txnKey]*txnPair
	dbMux sync.Mutex

	storage Storage

	cancel *atomic.Bool
}

func (s *stateExTxn) Commit() error {
	if s.cancel.Load() {
		return ErrStateTransactionIsCancel
	}

	s.dbMux.Lock()
	defer s.dbMux.Unlock()

	// double check
	if s.cancel.Load() {
		return ErrStateTransactionIsCancel
	}

	batch := s.storage.Batch()

	for _, pair := range s.db {
		err := batch.Set(pair.key, pair.value)

		if err != nil {
			return err
		}
	}

	return batch.Commit()
}

// other storage backend handle rollback in this function
func (s *stateExTxn) Rollback() {
	s.dbMux.Lock()
	defer s.dbMux.Unlock()

	if s.cancel.Load() {
		return
	}

	s.cancel.Store(true)
}

func (s *stateExTxn) Set(k []byte, v []byte) error {
	s.dbMux.Lock()
	defer s.dbMux.Unlock()

	bufKey := make([]byte, len(k))
	copy(bufKey[:], k[:])

	bufValue := make([]byte, len(v))
	copy(bufValue[:], v[:])

	s.db[txnKey(hex.EncodeToHex(k))] = &txnPair{
		key:   bufKey,
		value: bufValue,
	}

	return nil
}

func (s *stateExTxn) Get(k []byte) ([]byte, bool, error) {
	s.dbMux.Lock()
	defer s.dbMux.Unlock()

	v, ok := s.db[txnKey(hex.EncodeToHex(k))]
	if !ok {
		return []byte{}, false, nil
	}

	bufValue := make([]byte, len(v.value))
	copy(bufValue[:], v.value[:])

	return bufValue, true, nil
}

type stateDBImpl struct {
	logger hclog.Logger

	storage Storage

	cached *fastcache.Cache

	txnMux        sync.Mutex
	isTransaction *atomic.Bool

	stateExTxnRef *stateExTxn
}

func NewStateDB(storage Storage, logger hclog.Logger) StateDB {
	return &stateDBImpl{
		logger:        logger.Named("state"),
		storage:       storage,
		cached:        fastcache.New(32 * 1024 * 1024),
		isTransaction: atomic.NewBool(false),
		stateExTxnRef: nil,
	}
}

func (db *stateDBImpl) Set(k, v []byte) error {
	if db.isTransaction.Load() && db.stateExTxnRef != nil {
		return db.stateExTxnRef.Set(k, v)
	}

	err := db.storage.Set(k, v)
	if err != nil {
		return err
	}

	// update cache
	if db.cached != nil {
		db.cached.Set(k, v)
	}

	return nil
}

func (db *stateDBImpl) Get(k []byte) ([]byte, bool, error) {
	if db.isTransaction.Load() && db.stateExTxnRef != nil {
		v, ok, _ := db.stateExTxnRef.Get(k)
		if ok {
			return v, true, nil
		}
	}

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

func (db *stateDBImpl) SetCode(hash types.Hash, code []byte) error {
	if db.isTransaction.Load() && db.stateExTxnRef != nil {
		return db.stateExTxnRef.Set(append(codePrefix, hash.Bytes()...), code)
	}

	perfix := append(codePrefix, hash.Bytes()...)

	err := db.storage.Set(perfix, code)
	if err != nil {
		return nil
	}

	// update cache
	if db.cached != nil {
		db.cached.Set(perfix, code)
	}

	return nil
}

func (db *stateDBImpl) GetCode(hash types.Hash) ([]byte, bool) {
	if db.isTransaction.Load() && db.stateExTxnRef != nil {
		v, ok, _ := db.stateExTxnRef.Get(append(codePrefix, hash.Bytes()...))
		if ok {
			return v, true
		}
	}

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

func (db *stateDBImpl) ExclusiveTransaction(execute func(StateDBTransaction)) {
	db.txnMux.Lock()
	defer db.txnMux.Unlock()

	stateExTxnRef := &stateExTxn{
		db:      make(map[txnKey]*txnPair),
		storage: db.storage,
		cancel:  atomic.NewBool(false),
	}
	db.stateExTxnRef = stateExTxnRef

	db.isTransaction.Store(true)
	defer db.isTransaction.Store(false)

	execute(db.stateExTxnRef)

	db.stateExTxnRef = nil

	// update cache
	for _, pair := range stateExTxnRef.db {
		db.cached.Set(pair.key, pair.value)
	}
}
