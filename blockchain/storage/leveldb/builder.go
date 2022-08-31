package leveldb

import (
	"fmt"

	"github.com/dogechain-lab/dogechain/blockchain/storage"
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

func max(a, b int) int {
	if a > b {
		return a
	}

	return b
}

type Builder interface {
	// set cache size
	SetCacheSize(int) Builder

	// set handles
	SetHandles(int) Builder

	// set bloom key bits
	SetBloomKeyBits(int) Builder

	// set compaction table size
	SetCompactionTableSize(int) Builder

	// set compaction table total size
	SetCompactionTotalSize(int) Builder

	// set no sync
	SetNoSync(bool) Builder

	// build the storage
	Build() (storage.Storage, error)
}

type leveldbBuilder struct {
	logger  hclog.Logger
	path    string
	options *leveldbopt.Options
}

func (builder *leveldbBuilder) SetCacheSize(cacheSize int) Builder {
	cacheSize = max(cacheSize, minCache)

	builder.options.BlockCacheCapacity = (cacheSize / 2) * leveldbopt.MiB
	builder.options.WriteBuffer = (cacheSize / 4) * leveldbopt.MiB

	builder.logger.Info("leveldb",
		"BlockCacheCapacity", fmt.Sprintf("%d Mib", cacheSize/2),
		"WriteBuffer", fmt.Sprintf("%d Mib", cacheSize/4),
	)

	return builder
}

func (builder *leveldbBuilder) SetHandles(handles int) Builder {
	builder.options.OpenFilesCacheCapacity = max(handles, minHandles)

	builder.logger.Info("leveldb",
		"OpenFilesCacheCapacity", builder.options.OpenFilesCacheCapacity,
	)

	return builder
}

func (builder *leveldbBuilder) SetBloomKeyBits(bloomKeyBits int) Builder {
	builder.options.Filter = filter.NewBloomFilter(bloomKeyBits)

	builder.logger.Info("leveldb",
		"BloomFilter bits", bloomKeyBits,
	)

	return builder
}

func (builder *leveldbBuilder) SetCompactionTableSize(compactionTableSize int) Builder {
	builder.options.CompactionTableSize = compactionTableSize * leveldbopt.MiB

	builder.logger.Info("leveldb",
		"CompactionTableSize", fmt.Sprintf("%d Mib", compactionTableSize),
	)

	return builder
}

func (builder *leveldbBuilder) SetCompactionTotalSize(compactionTotalSize int) Builder {
	builder.options.CompactionTotalSize = compactionTotalSize * leveldbopt.MiB

	builder.logger.Info("leveldb",
		"CompactionTotalSize", fmt.Sprintf("%d Mib", compactionTotalSize),
	)

	return builder
}

func (builder *leveldbBuilder) SetNoSync(noSync bool) Builder {
	builder.options.NoSync = noSync

	builder.logger.Info("leveldb",
		"NoSync", noSync,
	)

	return builder
}

func (builder *leveldbBuilder) Build() (storage.Storage, error) {
	db, err := leveldb.OpenFile(builder.path, builder.options)
	if err != nil {
		return nil, err
	}

	kv := &levelDBKV{db}

	return storage.NewKeyValueStorage(builder.logger.Named("leveldb"), kv), nil
}

// NewBuilder creates the new leveldb storage builder
func NewBuilder(logger hclog.Logger, path string) Builder {
	return &leveldbBuilder{
		logger: logger,
		path:   path,
		options: &leveldbopt.Options{
			OpenFilesCacheCapacity: minHandles,
			CompactionTableSize:    DefaultCompactionTableSize * leveldbopt.MiB,
			CompactionTotalSize:    DefaultCompactionTotalSize * leveldbopt.MiB,
			BlockCacheCapacity:     minCache / 2 * leveldbopt.MiB,
			WriteBuffer:            minCache / 4 * leveldbopt.MiB,
			Filter:                 filter.NewBloomFilter(DefaultBloomKeyBits),
			NoSync:                 false,
		},
	}
}
