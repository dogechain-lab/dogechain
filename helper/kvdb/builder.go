package kvdb

import (
	"fmt"
	"math"

	"github.com/hashicorp/go-hclog"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

const (
	// base block size
	blockSize = 2 * opt.MiB // default 4kb, but one key-value pair need 0.5kb

	// minLevelDBCache is the minimum memory allocate to leveldb
	// half write, half read
	minLevelDBCache = 16 // 16 MiB

	// minLevelDBHandles is the minimum number of files handles to leveldb open files
	minLevelDBHandles = 16

	DefaultLevelDBCache               = 128  // 128 MiB
	DefaultLevelDBHandles             = 512  // files handles to leveldb open files
	DefaultLevelDBBloomKeyBits        = 2048 // bloom filter bits (256 bytes)
	DefaultLevelDBCompactionTableSize = 10   // 10 MiB
	DefaultLevelDBCompactionTotalSize = 100  // 100 MiB
	DefaultLevelDBNoSync              = false
)

func max(a, b int) int {
	if a > b {
		return a
	}

	return b
}

type LevelDBBuilder interface {
	// set cache size
	SetCacheSize(int) LevelDBBuilder

	// set handles
	SetHandles(int) LevelDBBuilder

	// set bloom key bits
	SetBloomKeyBits(int) LevelDBBuilder

	// set compaction table size
	SetCompactionTableSize(int) LevelDBBuilder

	// set compaction table total size
	SetCompactionTotalSize(int) LevelDBBuilder

	// set no sync
	SetNoSync(bool) LevelDBBuilder

	// build the storage
	Build() (KVBatchStorage, error)
}

type leveldbBuilder struct {
	logger  hclog.Logger
	path    string
	options *opt.Options
}

func (builder *leveldbBuilder) SetCacheSize(cacheSize int) LevelDBBuilder {
	cacheSize = max(cacheSize, minLevelDBCache)

	cache := cacheSize * opt.MiB
	cache = (cache / blockSize) * blockSize // align to blockSize

	builder.logger.Info("leveldb",
		"BlockCacheCapacity", fmt.Sprintf("%d Mib", cache/opt.MiB),
	)

	return builder
}

func (builder *leveldbBuilder) SetHandles(handles int) LevelDBBuilder {
	builder.options.OpenFilesCacheCapacity = max(handles, minLevelDBHandles)

	builder.logger.Info("leveldb",
		"OpenFilesCacheCapacity", builder.options.OpenFilesCacheCapacity,
	)

	return builder
}

func (builder *leveldbBuilder) SetBloomKeyBits(bloomKeyBits int) LevelDBBuilder {
	builder.options.Filter = filter.NewBloomFilter(bloomKeyBits)

	builder.logger.Info("leveldb",
		"BloomFilter bits", bloomKeyBits,
	)

	return builder
}

func (builder *leveldbBuilder) SetCompactionTableSize(compactionTableSize int) LevelDBBuilder {
	// align to blockSize
	builder.options.CompactionTableSize = ((compactionTableSize * opt.MiB) / blockSize) * blockSize
	builder.options.WriteBuffer = builder.options.CompactionTableSize * 2

	builder.logger.Info("leveldb",
		"CompactionTableSize", fmt.Sprintf("%d Mib", builder.options.CompactionTableSize/opt.MiB),
		"WriteBuffer", fmt.Sprintf("%d Mib", builder.options.WriteBuffer/opt.MiB),
	)

	return builder
}

func (builder *leveldbBuilder) SetCompactionTotalSize(compactionTotalSize int) LevelDBBuilder {
	// align to blockSize
	builder.options.CompactionTotalSize = ((compactionTotalSize * opt.MiB) / blockSize) * blockSize
	// check size less than CompactionTableSize
	if builder.options.CompactionTotalSize < builder.options.CompactionTableSize {
		builder.options.CompactionTotalSize = builder.options.CompactionTableSize *
			(DefaultLevelDBCompactionTotalSize / DefaultLevelDBCompactionTableSize)

		builder.logger.Info(
			"leveldb",
			"CompactionTotalSize less than CompactionTableSize, match CompactionTableSize",
			builder.options.CompactionTotalSize/opt.MiB,
		)
	}

	builder.logger.Info("leveldb",
		"CompactionTotalSize", fmt.Sprintf("%d Mib", compactionTotalSize),
	)

	return builder
}

func (builder *leveldbBuilder) SetNoSync(noSync bool) LevelDBBuilder {
	builder.options.NoSync = noSync

	builder.logger.Info("leveldb",
		"NoSync", noSync,
	)

	return builder
}

func (builder *leveldbBuilder) Build() (KVBatchStorage, error) {
	db, err := leveldb.OpenFile(builder.path, builder.options)
	if err != nil {
		return nil, err
	}

	return &levelDBKV{db: db}, nil
}

// NewBuilder creates the new leveldb storage builder
func NewLevelDBBuilder(logger hclog.Logger, path string) LevelDBBuilder {
	// default options
	// Calculate parameters according to block size
	// align to blockSize
	compactionTableSize := ((DefaultLevelDBCompactionTableSize * opt.MiB) * blockSize) / blockSize // 10 MiB
	compactionTotalSize := ((DefaultLevelDBCompactionTotalSize * opt.MiB) * blockSize) / blockSize // 100 MiB

	// cache
	minBlockCache := ((DefaultLevelDBCache * opt.MiB) * blockSize) / blockSize // 16 MiB
	minWriteBuffer := 2 * compactionTableSize                                  // 20 MiB

	// filter
	filterBaseLg := int(math.Max(12, math.Log2(float64(blockSize)))) // minimum 12

	return &leveldbBuilder{
		logger: logger,
		path:   path,
		options: &opt.Options{
			OpenFilesCacheCapacity:        minLevelDBHandles,
			CompactionTableSize:           compactionTableSize,
			CompactionTotalSize:           compactionTotalSize,
			BlockCacheCapacity:            minBlockCache,
			WriteBuffer:                   minWriteBuffer,
			CompactionTableSizeMultiplier: 1.1, // scale size up 1.1 multiple in next level
			Filter:                        filter.NewBloomFilter(DefaultLevelDBBloomKeyBits),
			NoSync:                        false,
			BlockSize:                     blockSize,
			FilterBaseLg:                  filterBaseLg,
		},
	}
}
