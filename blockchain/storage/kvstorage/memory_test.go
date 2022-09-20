package kvstorage

import (
	"testing"

	"github.com/dogechain-lab/dogechain/blockchain/storage"
	"github.com/hashicorp/go-hclog"
)

func TestMemoryStorage(t *testing.T) {
	t.Helper()

	f := func(t *testing.T) (storage.Storage, func()) {
		t.Helper()

		s, _ := NewMemoryStorageBuilder(hclog.NewNullLogger()).Build()

		return s, func() {}
	}
	storage.TestStorage(t, f)
}
