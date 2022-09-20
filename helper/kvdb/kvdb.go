package kvdb

type KVBatch interface {
	Set(k, v []byte)
	Write() error
}

// KVStorage is a k/v storage on memory or leveldb
type KVStorage interface {
	Set(k, v []byte) error
	Get(k []byte) ([]byte, bool, error)

	Close() error
}

// KVBatchStorage is a batch write for leveldb
type KVBatchStorage interface {
	KVStorage
	Batch() KVBatch
}
