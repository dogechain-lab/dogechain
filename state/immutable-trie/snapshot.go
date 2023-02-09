package itrie

import (
	"github.com/dogechain-lab/dogechain/crypto"
	"github.com/dogechain-lab/dogechain/state"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/umbracle/fastrlp"
)

type Snapshot struct {
	state *stateDBImpl
	trie  *Trie
}

func (s *Snapshot) GetStorage(addr types.Address, root types.Hash, rawkey types.Hash) (types.Hash, error) {
	var (
		err  error
		trie *Trie
	)

	if root == types.EmptyRootHash {
		trie = s.state.newTrie()
	} else {
		trie, err = s.state.newTrieAt(root)
		if err != nil {
			return types.Hash{}, err
		}
	}

	key := crypto.Keccak256(rawkey.Bytes())

	val, err := trie.Get(key)
	if err != nil {
		// something bad happen, should not continue
		return types.Hash{}, err
	} else if len(val) == 0 {
		// not found
		return types.Hash{}, nil
	}

	p := &fastrlp.Parser{}

	v, err := p.Parse(val)
	if err != nil {
		return types.Hash{}, err
	}

	res := []byte{}
	if res, err = v.GetBytes(res[:0]); err != nil {
		return types.Hash{}, err
	}

	return types.BytesToHash(res), nil
}

func (s *Snapshot) GetAccount(addr types.Address) (*state.Account, error) {
	key := crypto.Keccak256(addr.Bytes())

	data, err := s.trie.Get(key)
	if err != nil {
		return nil, err
	} else if data == nil {
		// not found
		return nil, nil
	}

	var account state.Account
	if err := account.UnmarshalRlp(data); err != nil {
		return nil, err
	}

	return &account, nil
}

func (s *Snapshot) GetCode(hash types.Hash) ([]byte, bool) {
	return s.state.GetCode(hash)
}

func (s *Snapshot) Commit(objs []*state.Object) (state.Snapshot, []byte, error) {
	trie, root, err := s.trie.Commit(objs)
	if err != nil {
		return nil, nil, err
	}

	return &Snapshot{trie: trie, state: s.state}, root, nil
}
