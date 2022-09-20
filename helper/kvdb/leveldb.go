package kvdb

import (
	"errors"

	"github.com/syndtr/goleveldb/leveldb"
)

type levelBatch struct {
	db    *leveldb.DB
	batch *leveldb.Batch
}

func (b *levelBatch) Set(k, v []byte) {
	b.batch.Put(k, v)
}

func (b *levelBatch) Write() error {
	return b.db.Write(b.batch, nil)
}

// levelDBKV is the leveldb implementation of the kv storage
type levelDBKV struct {
	db *leveldb.DB
}

func (kv *levelDBKV) Batch() KVBatch {
	return &levelBatch{db: kv.db, batch: &leveldb.Batch{}}
}

// Set sets the key-value pair in leveldb storage
func (kv *levelDBKV) Set(p []byte, v []byte) error {
	return kv.db.Put(p, v, nil)
}

// Get retrieves the key-value pair in leveldb storage
func (kv *levelDBKV) Get(p []byte) ([]byte, bool, error) {
	data, err := kv.db.Get(p, nil)
	if err != nil {
		if errors.Is(err, leveldb.ErrNotFound) {
			return nil, false, nil
		} else if errors.Is(err, leveldb.ErrClosed) {
			return nil, false, nil
		} else {
			panic(err)
		}
	}

	return data, true, nil
}

// Close closes the leveldb storage instance
func (kv *levelDBKV) Close() error {
	return kv.db.Close()
}
