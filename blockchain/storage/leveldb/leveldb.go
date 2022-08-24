package leveldb

import (
	"fmt"

	"github.com/dogechain-lab/dogechain/blockchain/storage"
	"github.com/hashicorp/go-hclog"
	"github.com/syndtr/goleveldb/leveldb"
	leveldbopt "github.com/syndtr/goleveldb/leveldb/opt"
)

const (
	// minCache is the minimum memory allocate to leveldb
	// half write, half read
	minCache = 16

	// minHandles is the minimum number of files handles to leveldb open files
	minHandles = 16

	DefaultCache   = 16  // 16 MiB
	DefaultHandles = 256 // files handles to leveldb open files
)

// Factory creates a leveldb storage
func Factory(config map[string]interface{}, logger hclog.Logger) (storage.Storage, error) {
	path, ok := config["path"]
	if !ok {
		return nil, fmt.Errorf("path not found")
	}

	pathStr, ok := path.(string)
	if !ok {
		return nil, fmt.Errorf("path is not a string")
	}

	cache, ok := config["cache"]
	if !ok {
		cache = minCache
	}

	cacheInt, ok := cache.(int)
	if !ok {
		return nil, fmt.Errorf("cache is not a int")
	}

	handles, ok := config["handles"]
	if !ok {
		handles = minHandles
	}

	handlesInt, ok := handles.(int)
	if !ok {
		return nil, fmt.Errorf("handles is not a int")
	}

	return NewLevelDBStorage(pathStr, cacheInt, handlesInt, logger)
}

// NewLevelDBStorage creates the new storage reference with leveldb
func NewLevelDBStorage(path string, cache int, handles int, logger hclog.Logger) (storage.Storage, error) {
	opt := &leveldbopt.Options{}

	if cache < minCache {
		cache = minCache
	}
	if handles < minHandles {
		handles = minHandles
	}

	opt.OpenFilesCacheCapacity = handles
	opt.BlockCacheCapacity = cache / 2 * leveldbopt.MiB
	opt.WriteBuffer = cache / 4 * leveldbopt.MiB

	db, err := leveldb.OpenFile(path, opt)
	if err != nil {
		return nil, err
	}

	kv := &levelDBKV{db}

	return storage.NewKeyValueStorage(logger.Named("leveldb"), kv), nil
}

// levelDBKV is the leveldb implementation of the kv storage
type levelDBKV struct {
	db *leveldb.DB
}

// Set sets the key-value pair in leveldb storage
func (l *levelDBKV) Set(p []byte, v []byte) error {
	return l.db.Put(p, v, nil)
}

// Get retrieves the key-value pair in leveldb storage
func (l *levelDBKV) Get(p []byte) ([]byte, bool, error) {
	data, err := l.db.Get(p, nil)
	if err != nil {
		if err.Error() == "leveldb: not found" {
			return nil, false, nil
		}

		return nil, false, err
	}

	return data, true, nil
}

// Close closes the leveldb storage instance
func (l *levelDBKV) Close() error {
	return l.db.Close()
}
