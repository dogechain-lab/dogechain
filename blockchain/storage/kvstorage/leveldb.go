package kvstorage

import (
	"github.com/dogechain-lab/dogechain/blockchain/storage"
	"github.com/dogechain-lab/dogechain/helper/kvdb"
	"github.com/hashicorp/go-hclog"
)

type leveldbStorageBuilder struct {
	logger         hclog.Logger
	leveldbBuilder kvdb.LevelDBBuilder
}

func (builder *leveldbStorageBuilder) Build() (storage.Storage, error) {
	db, err := builder.leveldbBuilder.Build()
	if err != nil {
		return nil, err
	}

	return newKeyValueStorage(builder.logger.Named("leveldb"), db), nil
}

// NewLevelDBStorageBuilder creates the new blockchain storage builder
func NewLevelDBStorageBuilder(logger hclog.Logger, leveldbBuilder kvdb.LevelDBBuilder) storage.StorageBuilder {
	return &leveldbStorageBuilder{
		logger:         logger,
		leveldbBuilder: leveldbBuilder,
	}
}
