package kvdb

import (
	"encoding/binary"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

func createTestDB(t *testing.T) KVBatchStorage {
	t.Helper()

	tempDir, err := ioutil.TempDir("/tmp", "leveldb-")
	assert.NoError(t, err)

	db, err := NewLevelDBBuilder(
		hclog.NewNullLogger(),
		tempDir,
	).Build()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	return db
}

func TestLevelDB(t *testing.T) {
	t.Parallel()

	t.Run("test KVStorage Get/Set", func(t *testing.T) {
		t.Parallel()

		db := createTestDB(t)
		defer db.Close()

		if err := db.Set([]byte("hello"), []byte("world")); err != nil {
			t.Fatal(err)
		}

		v, exist, err := db.Get([]byte("hello"))
		if err != nil {
			t.Fatal(err)
		}

		assert.True(t, exist)

		if string(v) != "world" {
			t.Fatal("value not equal")
		}
	})

	t.Run("test KVStorage Batch Write", func(t *testing.T) {
		t.Parallel()

		seed := rand.NewSource(time.Now().Unix())

		db := createTestDB(t)
		defer db.Close()

		keys := [][]byte{{}}
		values := [][]byte{{}}

		{
			batch := db.Batch()
			r := rand.New(seed)

			for i := 0; i < 100; i++ {
				key := make([]byte, 32)
				r.Read(key)

				keys = append(keys, key)

				value := make([]byte, 128)
				r.Read(value)

				values = append(values, value)

				batch.Set(key, value)
			}

			if err := batch.Write(); err != nil {
				t.Fatal(err)
			}
		}

		{
			for i := 1; i < len(keys); i++ {
				val, exist, err := db.Get(keys[i])
				if err != nil {
					t.Fatal(err)
				}

				assert.True(t, exist)
				assert.Equal(t, values[i], val)
			}
		}
	})

	t.Run("test KVStorage Iteration", func(t *testing.T) {
		t.Parallel()

		seed := rand.NewSource(time.Now().Unix())

		db := createTestDB(t)
		defer db.Close()

		keys := [][]byte{{}}
		values := [][]byte{{}}

		{
			batch := db.Batch()
			r := rand.New(seed)

			for i := 0; i < 100; i++ {
				key := make([]byte, 32)
				r.Read(key)

				prefix := make([]byte, 4)
				binary.LittleEndian.PutUint32(prefix, uint32(i))

				key = append(prefix[:], key...)

				keys = append(keys, key)

				value := make([]byte, 128)
				r.Read(value)

				values = append(values, value)

				batch.Set(key, value)
			}

			if err := batch.Write(); err != nil {
				t.Fatal(err)
			}
		}

		{
			iter := db.Iterator(nil)
			defer iter.Release()

			iter.First()

			for i := 1; i < len(keys); i++ {
				assert.Equal(t, keys[i], iter.Key())
				assert.Equal(t, values[i], iter.Value())

				iter.Next()
			}
		}
	})
}
