package kvdb

type KVBatch interface {
	Set(k, v []byte)
	Write() error
}

type KVIteratorRange struct {
	Start []byte
	Limit []byte
}

type KVIterator interface {
	// First moves the iterator to the first key/value pair. If the iterator
	// only contains one key/value pair then First and Last would moves
	// to the same key/value pair.
	// It returns whether such pair exist.
	First() bool

	// Last moves the iterator to the last key/value pair. If the iterator
	// only contains one key/value pair then First and Last would moves
	// to the same key/value pair.
	// It returns whether such pair exist.
	Last() bool

	// Seek moves the iterator to the first key/value pair whose key is greater
	// than or equal to the given key.
	// It returns whether such pair exist.
	//
	// It is safe to modify the contents of the argument after Seek returns.
	Seek(key []byte) bool

	// Next moves the iterator to the next key/value pair.
	// It returns false if the iterator is exhausted.
	Next() bool

	// Prev moves the iterator to the previous key/value pair.
	// It returns false if the iterator is exhausted.
	Prev() bool

	// Key returns the key of the current key/value pair, or nil if done.
	// The caller should not modify the contents of the returned slice, and
	// its contents may change on the next call to any 'seeks method'.
	Key() []byte

	// Value returns the value of the current key/value pair, or nil if done.
	// The caller should not modify the contents of the returned slice, and
	// its contents may change on the next call to any 'seeks method'.
	Value() []byte

	// Release releases associated resources. Release should always success
	// and can be called multiple times without causing error.
	Release()

	// Error returns any accumulated error. Exhausting all the key/value pairs
	// is not considered to be an error.
	Error() error
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

	Iterator(*KVIteratorRange) KVIterator

	Batch() KVBatch
}
