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
	codeCacheSize = 32 * 1024 * 1024 // 32MB

	trieStateLruCacheSize    = 128
	accountStateLruCacheSize = 2048
)

type State struct {
	storage Storage

	trieStateCache    *lru.Cache
	accountStateCache *lru.Cache

	codeCache *fastcache.Cache

	metrics *Metrics
}

func NewState(storage Storage, metrics *Metrics) *State {
	codeCache := fastcache.New(codeCacheSize * 1024 * 1024)

	trieStateCache, _ := lru.New(trieStateLruCacheSize)
	accountStateCache, _ := lru.New(accountStateLruCacheSize)

	s := &State{
		storage:           storage,
		trieStateCache:    trieStateCache,
		accountStateCache: accountStateCache,
		codeCache:         codeCache,
		metrics:           NewDummyMetrics(metrics),
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
	err := s.storage.SetCode(hash, code)

	if err == nil {
		s.codeCache.Set(hash.Bytes(), code)
		s.metrics.MemCacheWrite.Add(1)
	}

	return err
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

	tt, ok := s.trieStateCache.Get(root)
	if ok {
		trie, ok := tt.(*Trie)
		if !ok {
			return nil, errors.New("invalid type assertion")
		}

		s.metrics.TrieStateLruCacheHit.Add(1)

		return trie, nil
	}

	s.metrics.TrieStateLruCacheMiss.Add(1)

	tt, ok = s.accountStateCache.Get(root)
	if ok {
		trie, ok := tt.(*Trie)
		if !ok {
			return nil, errors.New("invalid type assertion")
		}

		s.metrics.AccountStateLruCacheHit.Add(1)

		return trie, nil
	}

	s.metrics.AccountStateLruCacheMiss.Add(1)

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

func (s *State) AddAccountState(root types.Hash, t *Trie) {
	s.accountStateCache.Add(root, t)
}

func (s *State) AddTrieState(root types.Hash, t *Trie) {
	s.trieStateCache.Add(root, t)
}
