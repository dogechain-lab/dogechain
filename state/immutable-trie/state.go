package itrie

import (
	"errors"
	"fmt"

	"github.com/VictoriaMetrics/fastcache"

	lru "github.com/hashicorp/golang-lru"

	"github.com/dogechain-lab/dogechain/state"
	"github.com/dogechain-lab/dogechain/types"
)

const (
	cacheSize = 32 * 1024 * 1024 // 32MB
)

type State struct {
	storage Storage

	cache     *lru.Cache
	codeCache *fastcache.Cache

	metrics *Metrics
}

func NewState(storage Storage, metrics *Metrics) *State {
	codeCache := fastcache.New(cacheSize)
	cache, _ := lru.New(128)

	s := &State{
		storage:   storage,
		cache:     cache,
		codeCache: codeCache,
		metrics:   NewDummyMetrics(metrics),
	}

	return s
}

func (s *State) NewSnapshot() state.Snapshot {
	t := NewTrie()
	t.state = s
	t.storage = s.storage

	return t
}

func (s *State) SetCode(hash types.Hash, code []byte) error {
	s.codeCache.Set(hash.Bytes(), code)
	s.metrics.MemCacheWrite.Add(1)

	return s.storage.SetCode(hash, code)
}

func (s *State) GetCode(hash types.Hash) ([]byte, bool) {
	defer s.metrics.MemCacheRead.Add(1)

	if enc := s.codeCache.Get(nil, hash.Bytes()); enc != nil {
		s.metrics.MemCacheHit.Add(1)

		return enc, true
	}

	s.metrics.MemCacheMiss.Add(1)

	code, ok := s.storage.GetCode(hash)
	if ok {
		s.codeCache.Set(hash.Bytes(), code)

		s.metrics.MemCacheWrite.Add(1)
	}

	return code, ok
}

func (s *State) NewSnapshotAt(root types.Hash) (state.Snapshot, error) {
	if root == types.EmptyRootHash {
		// empty state
		return s.NewSnapshot(), nil
	}

	tt, ok := s.cache.Get(root)
	if ok {
		t, ok := tt.(*Trie)
		if !ok {
			return nil, errors.New("invalid type assertion")
		}

		t.state = s

		trie, ok := tt.(*Trie)
		if !ok {
			return nil, errors.New("invalid type assertion")
		}

		return trie, nil
	}

	n, ok, err := GetNode(root.Bytes(), s.storage)

	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, fmt.Errorf("state not found at hash %s", root)
	}

	t := &Trie{
		root:    n,
		state:   s,
		storage: s.storage,
	}

	return t, nil
}

func (s *State) AddState(root types.Hash, t *Trie) {
	s.cache.Add(root, t)
}
