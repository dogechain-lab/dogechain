package leveldb

import (
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
	minCache = 16 // 16 MiB

	// minHandles is the minimum number of files handles to leveldb open files
	minHandles = 16

	DefaultCache               = 1024 // 1 GiB
	DefaultHandles             = 512  // files handles to leveldb open files
	DefaultBloomKeyBits        = 2048 // bloom filter bits (256 bytes)
	DefaultCompactionTableSize = 8    // 8  MiB
	DefaultCompactionTotalSize = 32   // 32 MiB
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

	logger.Info("leveldb",
		"OpenFilesCacheCapacity", opt.OpenFilesCacheCapacity,
		"CompactionTableSize", compactionTableSize,
		"BlockCacheCapacity", cache/2,
		"WriteBuffer", cache/4,
		"BloomFilter bits", bloomKeyBits,
		"NoSync", o.NoSync,
	)

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
