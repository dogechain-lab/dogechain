package itrie

import (
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"github.com/dogechain-lab/dogechain/state"
	"github.com/dogechain-lab/dogechain/types"
	"go.uber.org/atomic"
)

type StateDBTransaction interface {
	StateDBReader
	StorageWriter

	GetCode(hash types.Hash) ([]byte, bool)
	SetCode(hash types.Hash, code []byte) error

	Commit() error
	Rollback()
}

type txnKey string
type txnPair struct {
	key    []byte
	value  []byte
	isCode bool
}

var txnPairPool = sync.Pool{
	New: func() interface{} {
		return &txnPair{
			key:    make([]byte, 0),
			value:  make([]byte, 0),
			isCode: false,
		}
	},
}

func (pair *txnPair) Reset() {
	pair.key = pair.key[0:0]
	pair.value = pair.value[0:0]
	pair.isCode = false
}

type stateDBTxn struct {
	db   map[txnKey]*txnPair
	lock sync.Mutex

	stateDB StateDB
	storage Storage

	cancel *atomic.Bool
}

func (tx *stateDBTxn) Set(k []byte, v []byte) error {
	tx.lock.Lock()
	defer tx.lock.Unlock()

	pair, ok := txnPairPool.Get().(*txnPair)
	if !ok {
		return errors.New("invalid type assertion")
	}

	pair.key = append(pair.key, k...)
	pair.value = append(pair.value, v...)

	tx.db[txnKey(hex.EncodeToString(k))] = pair

	return nil
}

func (tx *stateDBTxn) Get(k []byte) ([]byte, bool, error) {
	tx.lock.Lock()
	defer tx.lock.Unlock()

	v, ok := tx.db[txnKey(hex.EncodeToString(k))]
	if !ok {
		return tx.stateDB.Get(k)
	}

	bufValue := make([]byte, len(v.value))
	copy(bufValue[:], v.value[:])

	return bufValue, true, nil
}

func (tx *stateDBTxn) SetCode(hash types.Hash, v []byte) error {
	tx.lock.Lock()
	defer tx.lock.Unlock()

	perfix := append(codePrefix, hash.Bytes()...)

	pair, ok := txnPairPool.Get().(*txnPair)
	if !ok {
		return errors.New("invalid type assertion")
	}

	pair.key = append(pair.key, perfix...)
	pair.value = append(pair.value, v...)
	pair.isCode = true

	tx.db[txnKey(hex.EncodeToString(perfix))] = pair

	return nil
}

func (tx *stateDBTxn) GetCode(hash types.Hash) ([]byte, bool) {
	tx.lock.Lock()
	defer tx.lock.Unlock()

	perfix := append(codePrefix, hash.Bytes()...)

	v, ok := tx.db[txnKey(hex.EncodeToString(perfix))]
	if !ok {
		return tx.stateDB.GetCode(hash)
	}

	// depth copy
	bufValue := make([]byte, len(v.value))
	copy(bufValue[:], v.value[:])

	return bufValue, true
}

func (tx *stateDBTxn) NewSnapshot() state.Snapshot {
	return tx.stateDB.NewSnapshot()
}

func (tx *stateDBTxn) NewSnapshotAt(root types.Hash) (state.Snapshot, error) {
	if root == types.EmptyRootHash {
		// empty state
		return tx.NewSnapshot(), nil
	}

	// user exclusive transaction to get state
	// use non-commit state
	n, ok, err := GetNode(root.Bytes(), tx)

	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, fmt.Errorf("state not found at hash %s", root)
	}

	t := NewTrie()
	t.root = n
	t.stateDB = tx.stateDB

	return t, nil
}

func (tx *stateDBTxn) Commit() error {
	if tx.cancel.Load() {
		return ErrStateTransactionIsCancel
	}

	tx.lock.Lock()
	defer tx.lock.Unlock()

	// double check
	if tx.cancel.Load() {
		return ErrStateTransactionIsCancel
	}

	batch := tx.storage.NewBatch()
	metrics := tx.stateDB.GetMetrics()

	for _, pair := range tx.db {
		err := batch.Set(pair.key, pair.value)

		if err != nil {
			return err
		}

		if !pair.isCode {
			metrics.transactionWriteNodeSizeObserve(len(pair.value))
		}
	}

	return batch.Commit()
}

// clear transaction data, set cancel flag
func (tx *stateDBTxn) Rollback() {
	tx.lock.Lock()
	defer tx.lock.Unlock()

	if tx.cancel.Load() {
		return
	}

	tx.cancel.Store(true)

	tx.clear()
}

func (tx *stateDBTxn) clear() {
	tx.stateDB = nil
	tx.storage = nil

	for tk := range tx.db {
		pair := tx.db[tk]
		pair.Reset()

		txnPairPool.Put(pair)
		delete(tx.db, tk)
	}
}
