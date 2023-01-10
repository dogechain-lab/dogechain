package itrie

import (
	"fmt"

	"github.com/dogechain-lab/dogechain/helper/hex"
	"github.com/dogechain-lab/dogechain/helper/kvdb"
	"github.com/dogechain-lab/fastrlp"
)

var parserPool fastrlp.ParserPool

// Storage stores the trie
type StorageReader interface {
	Get(k []byte) ([]byte, bool, error)
}

type StorageWriter interface {
	Set(k, v []byte) error
}

type Batch interface {
	StorageWriter

	Commit() error
}

// Storage stores the trie
type Storage interface {
	StorageReader
	StorageWriter

	NewBatch() Batch
	Close() error
}

type kvStorageBatch struct {
	batch kvdb.KVBatch
}

func (kvBatch *kvStorageBatch) Set(k, v []byte) error {
	kvBatch.batch.Set(k, v)

	return nil
}

func (kvBatch *kvStorageBatch) Commit() error {
	return kvBatch.batch.Write()
}

// wrap generic kvdb storage to implement Storage interface
type kvStorage struct {
	db kvdb.KVBatchStorage
}

func (kv *kvStorage) Get(k []byte) ([]byte, bool, error) {
	return kv.db.Get(k)
}

func (kv *kvStorage) Set(k, v []byte) error {
	return kv.db.Set(k, v)
}

func (kv *kvStorage) NewBatch() Batch {
	return &kvStorageBatch{
		batch: kv.db.Batch(),
	}
}

func (kv *kvStorage) Close() error {
	return kv.db.Close()
}

func NewLevelDBStorage(leveldbBuilder kvdb.LevelDBBuilder) (Storage, error) {
	db, err := leveldbBuilder.Build()
	if err != nil {
		return nil, err
	}

	return &kvStorage{db: db}, nil
}

type memStorage struct {
	db map[string][]byte
}

type memBatch struct {
	db *map[string][]byte
}

// NewMemoryStorage creates an inmemory trie storage
func NewMemoryStorage() Storage {
	return &memStorage{db: map[string][]byte{}}
}

func (m *memStorage) Set(p []byte, v []byte) error {
	buf := make([]byte, len(v))
	copy(buf[:], v[:])
	m.db[hex.EncodeToHex(p)] = buf

	return nil
}

func (m *memStorage) Get(p []byte) ([]byte, bool, error) {
	v, ok := m.db[hex.EncodeToHex(p)]
	if !ok {
		return []byte{}, false, nil
	}

	return v, true, nil
}

func (m *memStorage) NewBatch() Batch {
	return &memBatch{db: &m.db}
}

func (m *memStorage) Close() error {
	return nil
}

func (m *memBatch) Set(p, v []byte) error {
	buf := make([]byte, len(v))
	copy(buf[:], v[:])
	(*m.db)[hex.EncodeToHex(p)] = buf

	return nil
}

func (m *memBatch) Commit() error {
	return nil
}

// GetNode retrieves a node from storage
func GetNode(root []byte, storage StorageReader) (Node, bool, error) {
	data, ok, _ := storage.Get(root)
	if !ok {
		return nil, false, nil
	}

	// NOTE. We dont need to make copies of the bytes because the nodes
	// take the reference from data itself which is a safe copy.
	p := parserPool.Get()
	defer parserPool.Put(p)

	v, err := p.Parse(data)
	if err != nil {
		return nil, false, err
	}

	if v.Type() != fastrlp.TypeArray {
		return nil, false, fmt.Errorf("storage item should be an array")
	}

	n, err := decodeNode(v)

	return n, err == nil, err
}

func decodeNode(v *fastrlp.Value) (Node, error) {
	if v.Type() == fastrlp.TypeBytes {
		vv := nodePool.GetValueNode()
		vv.hash = true
		vv.buf = append(vv.buf[0:0], v.Raw()...)

		return vv, nil
	}

	var err error

	// TODO remove this once 1.0.4 of ifshort is merged in golangci-lint
	ll := v.Elems() //nolint:ifshort
	if ll == 2 {
		key := v.Get(0)
		if key.Type() != fastrlp.TypeBytes {
			return nil, fmt.Errorf("short key expected to be bytes")
		}

		// this can be either an array (extension node)
		// or bytes (leaf node)
		nc := nodePool.GetShortNode()
		nc.key = decodeCompact(key.Raw())

		if hasTerminator(nc.key) {
			// value node
			if v.Get(1).Type() != fastrlp.TypeBytes {
				return nil, fmt.Errorf("short leaf value expected to be bytes")
			}

			vv := nodePool.GetValueNode()
			vv.buf = append(vv.buf, v.Get(1).Raw()...)
			nc.child = vv
		} else {
			nc.child, err = decodeNode(v.Get(1))
			if err != nil {
				return nil, err
			}
		}

		return nc, nil
	} else if ll == 17 {
		// full node
		nc := nodePool.GetFullNode()
		for i := 0; i < 16; i++ {
			if v.Get(i).Type() == fastrlp.TypeBytes && len(v.Get(i).Raw()) == 0 {
				// empty
				continue
			}
			nc.children[i], err = decodeNode(v.Get(i))
			if err != nil {
				return nil, err
			}
		}

		if v.Get(16).Type() != fastrlp.TypeBytes {
			return nil, fmt.Errorf("full node value expected to be bytes")
		}
		if len(v.Get(16).Raw()) != 0 {
			vv := nodePool.GetValueNode()
			vv.buf = append(vv.buf[0:0], v.Get(16).Raw()...)
			nc.value = vv
		}

		return nc, nil
	}

	return nil, fmt.Errorf("node has incorrect number of leafs")
}
