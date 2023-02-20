package itrie

import (
	"bytes"
	"fmt"

	"github.com/dogechain-lab/dogechain/crypto"
	"github.com/dogechain-lab/dogechain/state"
	"github.com/dogechain-lab/dogechain/state/stypes"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/dogechain-lab/fastrlp"
)

type Snapshot struct {
	stateDB StateDB
	trie    *Trie
}

func newSnapshotImpl(stateDB StateDB, trie *Trie) *Snapshot {
	return &Snapshot{
		stateDB: stateDB,
		trie:    trie,
	}
}

func (s *Snapshot) GetStorage(addr types.Address, root types.Hash, rawkey types.Hash) (types.Hash, error) {
	var (
		err error
		ss  state.Snapshot
	)

	if root == types.EmptyRootHash {
		ss = s.stateDB.NewSnapshot()
	} else {
		// a new Snapshot on target contract state root
		ss, err = s.stateDB.NewSnapshotAt(root)
		if err != nil {
			return types.Hash{}, err
		}
	}

	// tricky downcast, but break out recursion
	snapshot, ok := ss.(*Snapshot)
	if !ok {
		return types.Hash{}, fmt.Errorf("invalid type assertion to Snapshot at %s", root)
	}

	// slot to hash
	key := crypto.Keccak256(rawkey.Bytes())

	val, err := snapshot.trie.Get(key, s.stateDB)
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

func (s *Snapshot) GetAccount(addr types.Address) (*stypes.Account, error) {
	key := addressHash(addr)

	data, err := s.trie.Get(key, s.stateDB)
	if err != nil {
		return nil, err
	} else if data == nil {
		// not found
		return nil, nil
	}

	var account stypes.Account
	if err := account.UnmarshalRlp(data); err != nil {
		return nil, err
	}

	return &account, nil
}

func (s *Snapshot) GetCode(hash types.Hash) ([]byte, bool) {
	return s.stateDB.GetCode(hash)
}

func (s *Snapshot) Commit(objs []*stypes.Object) (state.Snapshot, []byte, error) {
	var (
		root  []byte = nil
		nTrie *Trie  = nil

		// metrics logger
		metrics         = s.stateDB.GetMetrics()
		insertCount     = 0
		deleteCount     = 0
		newSetCodeCount = 0
	)

	// Create an insertion batch for all the entries
	err := s.stateDB.Transaction(func(st StateDBTransaction) error {
		defer st.Rollback()

		tt := s.trie.Txn(st)

		arena := fastrlp.DefaultArenaPool.Get()
		defer fastrlp.DefaultArenaPool.Put(arena)

		ar1 := fastrlp.DefaultArenaPool.Get()
		defer fastrlp.DefaultArenaPool.Put(ar1)

		for _, obj := range objs {
			if obj.Deleted {
				// address hash
				err := tt.Delete(addressHash(obj.Address))
				if err != nil {
					return err
				}

				deleteCount++
			} else {
				account := stypes.Account{
					Balance:     obj.Balance,
					Nonce:       obj.Nonce,
					CodeHash:    obj.CodeHash.Bytes(),
					StorageRoot: obj.Root, // old root
				}

				if len(obj.Storage) != 0 {
					// last root
					rootsnap, err := st.NewSnapshotAt(obj.Root)
					// s.state.newTrieAt(obj.Root)
					if err != nil {
						return err
					}

					// tricky, but necessary here
					loadSnap, _ := rootsnap.(*Snapshot)
					// create a new Txn since we don't know whether there is any cache in it
					localTxn := loadSnap.trie.Txn(loadSnap.stateDB)

					for _, entry := range obj.Storage {
						// slot hash
						k := crypto.Keccak256(entry.Key)
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

					account.StorageRoot = types.BytesToHash(accountStateRoot)

					// update object state root, so that we could use it later
					obj.Root = account.StorageRoot
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

				tt.Insert(addressHash(obj.Address), data)
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

	return newSnapshotImpl(s.stateDB, nTrie), root, err
}
