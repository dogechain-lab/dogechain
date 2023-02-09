package itrie

import (
	"bytes"

	"github.com/dogechain-lab/dogechain/crypto"
	"github.com/dogechain-lab/dogechain/state"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/dogechain-lab/fastrlp"
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
	var (
		root  []byte = nil
		nTrie *Trie  = nil

		// metrics logger
		metrics         = s.state.GetMetrics()
		insertCount     = 0
		deleteCount     = 0
		newSetCodeCount = 0
	)

	// Create an insertion batch for all the entries
	err := s.state.Transaction(func(st StateDBTransaction) error {
		defer st.Rollback()

		tt := s.trie.Txn()
		tt.reader = st

		arena := fastrlp.DefaultArenaPool.Get()
		defer fastrlp.DefaultArenaPool.Put(arena)

		ar1 := fastrlp.DefaultArenaPool.Get()
		defer fastrlp.DefaultArenaPool.Put(ar1)

		for _, obj := range objs {
			if obj.Deleted {
				err := tt.Delete(hashit(obj.Address.Bytes()))
				if err != nil {
					return err
				}

				deleteCount++
			} else {
				account := state.Account{
					Balance:  obj.Balance,
					Nonce:    obj.Nonce,
					CodeHash: obj.CodeHash.Bytes(),
					Root:     obj.Root, // old root
				}

				if len(obj.Storage) != 0 {
					trie, err := s.state.newTrieAt(obj.Root)
					if err != nil {
						return err
					}

					localTxn := trie.Txn()

					for _, entry := range obj.Storage {
						k := hashit(entry.Key)
						if entry.Deleted {
							err := localTxn.Delete(k)
							if err != nil {
								return err
							}

							deleteCount++
						} else {
							vv := ar1.NewBytes(bytes.TrimLeft(entry.Val, "\x00"))
							err := localTxn.Insert(k, vv.MarshalTo(nil))
							if err != nil {
								return err
							}

							insertCount++
						}
					}

					// observe account hash time
					observe := metrics.transactionAccountHashSecondsObserve()

					// write local trie to the storage
					accountStateRoot, _ := localTxn.Hash(st)

					// end observe account hash time
					observe()

					account.Root = types.BytesToHash(accountStateRoot)
				}

				if obj.DirtyCode {
					// write code to memory object, never failed
					// if failed, can't alloc memory, it will panic
					err := st.SetCode(obj.CodeHash, obj.Code)
					if err != nil {
						return err
					}

					newSetCodeCount++
				}

				vv := account.MarshalWith(arena)
				data := vv.MarshalTo(nil)

				tt.Insert(hashit(obj.Address.Bytes()), data)
				insertCount++

				arena.Reset()
			}
		}

		var err error

		// observe root hash time
		observe := metrics.transactionAccountHashSecondsObserve()

		root, err = tt.Hash(st)
		if err != nil {
			return err
		}

		// end observe root hash time
		observe()

		// dont use st here, we need to use the original stateDB
		nTrie = NewTrie()
		nTrie.stateDB = s.state
		nTrie.root = tt.root
		nTrie.epoch = tt.epoch

		// Commit all the entries to db
		return st.Commit()
	})

	if err == nil {
		metrics.transactionInsertObserve(insertCount)
		metrics.transactionDeleteObserve(deleteCount)
		metrics.transactionNewAccountObserve(newSetCodeCount)
	}

	return &Snapshot{trie: nTrie, state: s.state}, root, err
}
