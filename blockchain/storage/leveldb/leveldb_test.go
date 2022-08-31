package leveldb

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/dogechain-lab/dogechain/blockchain/storage"
	"github.com/hashicorp/go-hclog"
)

func newStorage(t *testing.T) (storage.Storage, func()) {
	t.Helper()

	path, err := ioutil.TempDir("/tmp", "minimal_storage")
	if err != nil {
		t.Fatal(err)
	}

	s, err := NewBuilder(
		hclog.NewNullLogger(), path).Build()
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

func TestStorage(t *testing.T) {
	storage.TestStorage(t, newStorage)
}
