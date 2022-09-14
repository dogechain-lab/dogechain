package kvstorage

import (
	"github.com/dogechain-lab/dogechain/blockchain/storage"
	"github.com/dogechain-lab/dogechain/helper/hex"
	"github.com/hashicorp/go-hclog"
)

type memoryStorageBuilder struct {
	logger hclog.Logger
}

func (builder *memoryStorageBuilder) Build() (storage.Storage, error) {
	db := &memoryKV{map[string][]byte{}}

	return newKeyValueStorage(builder.logger, db), nil
}

// NewMemoryStorageBuilder creates the new blockchain storage builder
func NewMemoryStorageBuilder(logger hclog.Logger) storage.StorageBuilder {
	return &memoryStorageBuilder{
		logger: logger,
	}
}

// memoryKV is an in memory implementation of the kv storage
type memoryKV struct {
	db map[string][]byte
}

func (m *memoryKV) Set(p []byte, v []byte) error {
	m.db[hex.EncodeToHex(p)] = v

	return nil
}

func (m *memoryKV) Get(p []byte) ([]byte, bool, error) {
	v, ok := m.db[hex.EncodeToHex(p)]
	if !ok {
		return nil, false, nil
	}

	return v, true, nil
}

func (m *memoryKV) Close() error {
	return nil
}
