package leveldb

import (
	"fmt"

	"github.com/dogechain-lab/dogechain/blockchain/storage"
	"github.com/dogechain-lab/dogechain/helper/common"
	"github.com/hashicorp/go-hclog"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/filter"
	leveldbopt "github.com/syndtr/goleveldb/leveldb/opt"
)

const (
	// minCache is the minimum memory allocate to leveldb
	// half write, half read
	minCache = 16

	// minHandles is the minimum number of files handles to leveldb open files
	minHandles = 16

	DefaultCache   = 16  // 16 MiB
	DefaultHandles = 512 // files handles to leveldb open files

	DefaultBloomKeyBits = 16 // bloom filter bits

	DefaultCompactionTableSize = 2  // 2 MiB
	DefaultCompactionTotalSize = 10 // 10 MiB
	DefaultNoSync              = false
)

type Options struct {
	CacheSize           int
	Handles             int
	BloomKeyBits        int
	CompactionTableSize int
	CompactionTotalSize int
	NoSync              bool
}

func NewDefaultOptons() *Options {
	return &Options{
		CacheSize:           minCache,
		Handles:             minHandles,
		BloomKeyBits:        DefaultBloomKeyBits,
		CompactionTableSize: DefaultCompactionTableSize,
		CompactionTotalSize: DefaultCompactionTotalSize,
		NoSync:              DefaultNoSync,
	}
}

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

	// get the options
	opt := NewDefaultOptons()

	cache, ok := config["cache"]
	if ok {
		opt.CacheSize, ok = cache.(int)
		if !ok {
			return nil, fmt.Errorf("cache is not a int")
		}
	}

	handles, ok := config["handles"]
	if ok {
		opt.Handles, ok = handles.(int)
		if !ok {
			return nil, fmt.Errorf("handles is not a int")
		}
	}

	bloomKeyBits, ok := config["bloomKeyBits"]
	if ok {
		opt.BloomKeyBits, ok = bloomKeyBits.(int)
		if !ok {
			return nil, fmt.Errorf("bloomKeyBits is not a int")
		}
	}

	compactionTableSize, ok := config["compactionTableSize"]
	if ok {
		opt.CompactionTableSize, ok = compactionTableSize.(int)
		if !ok {
			return nil, fmt.Errorf("compactionTableSize is not a int")
		}
	}

	compactionTotalSize, ok := config["compactionTotalSize"]
	if ok {
		opt.CompactionTotalSize, ok = compactionTotalSize.(int)
		if !ok {
			return nil, fmt.Errorf("compactionTableSize is not a int")
		}
	}

	nosync, ok := config["nosync"]
	if ok {
		opt.NoSync, ok = nosync.(bool)
		if !ok {
			return nil, fmt.Errorf("nosync is not a bool")
		}
	}

	return NewLevelDBStorage(pathStr, opt, logger)
}

// NewLevelDBStorage creates the new storage reference with leveldb
func NewLevelDBStorage(path string, o *Options, logger hclog.Logger) (storage.Storage, error) {
	opt := &leveldbopt.Options{}
	cache := o.CacheSize
	handles := o.Handles
	bloomKeyBits := o.BloomKeyBits
	compactionTableSize := o.CompactionTableSize
	compactionTotalSize := o.CompactionTotalSize

	if o.CacheSize < minCache {
		cache = minCache
	}
	if handles < minHandles {
		handles = minHandles
	}
	if compactionTableSize < DefaultCompactionTableSize {
		compactionTableSize = DefaultCompactionTableSize
	}
	if compactionTotalSize < DefaultCompactionTotalSize ||
		compactionTotalSize < compactionTableSize {
		compactionTotalSize = int(common.Max(
			uint64(DefaultCompactionTotalSize),
			uint64(compactionTableSize),
		))
	}

	opt.OpenFilesCacheCapacity = handles
	opt.CompactionTableSize = compactionTableSize * leveldbopt.MiB
	opt.CompactionTotalSize = compactionTotalSize * leveldbopt.MiB
	opt.BlockCacheCapacity = cache / 2 * leveldbopt.MiB
	opt.WriteBuffer = cache / 4 * leveldbopt.MiB
	opt.Filter = filter.NewBloomFilter(bloomKeyBits)
	opt.NoSync = o.NoSync

	logger.Info("leveldb", "OpenFilesCacheCapacity", opt.OpenFilesCacheCapacity)
	logger.Info("leveldb", "CompactionTableSize", compactionTableSize)
	logger.Info("leveldb", "BlockCacheCapacity", cache/2)
	logger.Info("leveldb", "WriteBuffer", cache/4)
	logger.Info("leveldb", "BloomFilter bits", bloomKeyBits)
	logger.Info("leveldb", "NoSync", o.NoSync)

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
