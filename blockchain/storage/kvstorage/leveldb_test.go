package kvstorage

import (
	"os"
	"testing"

	"github.com/dogechain-lab/dogechain/blockchain/storage"
	"github.com/dogechain-lab/dogechain/helper/kvdb"
	"github.com/hashicorp/go-hclog"
)

func newLevelDBStorage(t *testing.T) (storage.Storage, func()) {
	t.Helper()

	path, err := os.MkdirTemp("/tmp", "minimal_storage")
	if err != nil {
		t.Fatal(err)
	}

	logger := hclog.NewNullLogger()

	s, err := NewLevelDBStorageBuilder(
		logger, kvdb.NewLevelDBBuilder(logger, path)).Build()
	if err != nil {
		t.Fatal(err)
	}

	closeFn := func() {
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}

		if err := os.RemoveAll(path); err != nil {
			t.Fatal(err)
		}
	}

	return s, closeFn
}

func TestLevelDBStorage(t *testing.T) {
	storage.TestStorage(t, newLevelDBStorage)
}
