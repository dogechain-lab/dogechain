package leveldb

import (
	"github.com/dogechain-lab/dogechain/blockchain"
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

	return storage.NewKeyValueStorage(builder.logger.Named("leveldb"), db), nil
}

// NewBlockchainStorageBuilder creates the new blockchain storage builder
func NewBlockchainStorageBuilder(logger hclog.Logger, leveldbBuilder kvdb.LevelDBBuilder) blockchain.StorageBuilder {
	return &leveldbStorageBuilder{
		logger:         logger,
		leveldbBuilder: leveldbBuilder,
	}
}
