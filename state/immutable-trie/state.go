package itrie

import (
	"errors"
	"fmt"
	"sync"

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

type StateTransaction interface {
	Commit() error
	Rollback()
}

type State interface {
	StorageReader
	StorageWriter

	SetCode(hash types.Hash, code []byte) error
	GetCode(hash types.Hash) ([]byte, bool)

	NewSnapshot() state.Snapshot
	NewSnapshotAt(types.Hash) (state.Snapshot, error)

	ExclusiveTransaction(execute func(st StateTransaction))
	ExistTransaction() bool
}

type txnKey string
type txnPair struct {
	key   []byte
	value []byte
}

type stateExTxn struct {
	db    map[txnKey]*txnPair
	dbMux *sync.Mutex

	storage Storage
	cancel  *atomic.Bool
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

type stateImpl struct {
	logger hclog.Logger

	storage Storage

	txnMux        *sync.Mutex
	isTransaction *atomic.Bool

	stateExTxnMux *sync.Mutex
	stateExTxnRef *stateExTxn
}

func NewState(storage Storage, logger hclog.Logger) State {
	return &stateImpl{
		logger: logger.Named("state"),

		storage: storage,

		txnMux:        &sync.Mutex{},
		isTransaction: atomic.NewBool(false),

		stateExTxnMux: &sync.Mutex{},
		stateExTxnRef: nil,
	}
}

func (s *stateImpl) Set(k, v []byte) error {
	if s.isTransaction.Load() {
		s.stateExTxnMux.Lock()
		defer s.stateExTxnMux.Unlock()

		if s.stateExTxnRef != nil {
			_ = s.stateExTxnRef.Set(k, v)

			return nil
		}
	}

	return s.storage.Set(k, v)
}

func (s *stateImpl) Get(k []byte) ([]byte, bool, error) {
	if s.isTransaction.Load() {
		s.stateExTxnMux.Lock()
		defer s.stateExTxnMux.Unlock()

		if s.stateExTxnRef != nil {
			v, ok, _ := s.stateExTxnRef.Get(k)
			if ok {
				return v, true, nil
			}
		}
	}

	return s.storage.Get(k)
}

func (s *stateImpl) SetCode(hash types.Hash, code []byte) error {
	if s.isTransaction.Load() {
		s.stateExTxnMux.Lock()
		defer s.stateExTxnMux.Unlock()

		if s.stateExTxnRef != nil {
			_ = s.stateExTxnRef.Set(append(codePrefix, hash.Bytes()...), code)

			return nil
		}
	}

	return s.storage.Set(append(codePrefix, hash.Bytes()...), code)
}

func (s *stateImpl) GetCode(hash types.Hash) ([]byte, bool) {
	if s.isTransaction.Load() {
		s.stateExTxnMux.Lock()
		defer s.stateExTxnMux.Unlock()

		if s.stateExTxnRef != nil {
			v, ok, _ := s.stateExTxnRef.Get(append(codePrefix, hash.Bytes()...))
			if ok {
				return v, true
			}
		}
	}

	v, ok, err := s.storage.Get(append(codePrefix, hash.Bytes()...))
	if err != nil {
		s.logger.Error("get code", "err", err)
	}

	if !ok {
		return []byte{}, false
	}

	return v, true
}

func (s *stateImpl) NewSnapshot() state.Snapshot {
	t := NewTrie()
	t.state = s

	return t
}

func (s *stateImpl) NewSnapshotAt(root types.Hash) (state.Snapshot, error) {
	if root == types.EmptyRootHash {
		// empty state
		return s.NewSnapshot(), nil
	}

	n, ok, err := GetNode(root.Bytes(), s)

	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, fmt.Errorf("state not found at hash %s", root)
	}

	t := &Trie{
		root:  n,
		state: s,
	}

	return t, nil
}

func (s *stateImpl) ExclusiveTransaction(execute func(StateTransaction)) {
	// lock, only use StateTransaction.Cancel unlock
	s.txnMux.Lock()
	defer s.txnMux.Unlock()
	s.isTransaction.Store(true)

	s.stateExTxnRef = &stateExTxn{
		db:    make(map[txnKey]*txnPair),
		dbMux: &sync.Mutex{},

		storage: s.storage,
		cancel:  atomic.NewBool(false),
	}

	s.isTransaction.Store(true)
	defer s.isTransaction.Store(false)

	execute(s.stateExTxnRef)

	s.stateExTxnRef = nil
}

func (s *stateImpl) ExistTransaction() bool {
	return s.isTransaction.Load()
}
