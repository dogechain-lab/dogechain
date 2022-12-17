package itrie

import (
	"bytes"
	"errors"

	"github.com/dogechain-lab/dogechain/state"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/dogechain-lab/fastrlp"
	"golang.org/x/crypto/sha3"
)

type Trie struct {
	stateDB StateDB
	root    Node

	epoch uint32
}

func NewTrie() *Trie {
	return &Trie{}
}

func (t *Trie) Get(k []byte) ([]byte, bool) {
	txn := t.Txn()
	res := txn.Lookup(k)

	return res, res != nil
}

func hashit(k []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(k)

	return h.Sum(nil)
}

var accountArenaPool fastrlp.ArenaPool

var stateArenaPool fastrlp.ArenaPool // TODO, Remove once we do update in fastrlp

func (t *Trie) Commit(objs []*state.Object) (state.Snapshot, []byte, error) {
	var root []byte = nil

	var nTrie *Trie = nil

	// Create an insertion batch for all the entries
	err := t.stateDB.Transaction(func(st StateDBTransaction) error {
		defer st.Rollback()

		tt := t.Txn()
		tt.reader = st

		arena := accountArenaPool.Get()
		defer accountArenaPool.Put(arena)

		ar1 := stateArenaPool.Get()
		defer stateArenaPool.Put(ar1)

		for _, obj := range objs {
			if obj.Deleted {
				tt.Delete(hashit(obj.Address.Bytes()))
			} else {
				account := state.Account{
					Balance:  obj.Balance,
					Nonce:    obj.Nonce,
					CodeHash: obj.CodeHash.Bytes(),
					Root:     obj.Root, // old root
				}

				if len(obj.Storage) != 0 {
					localSnapshot, err := t.stateDB.NewSnapshotAt(obj.Root)
					if err != nil {
						panic(err)
					}

					trie, ok := localSnapshot.(*Trie)
					if !ok {
						return errors.New("invalid type assertion")
					}

					localTxn := trie.Txn()

					for _, entry := range obj.Storage {
						k := hashit(entry.Key)
						if entry.Deleted {
							localTxn.Delete(k)
						} else {
							vv := ar1.NewBytes(bytes.TrimLeft(entry.Val, "\x00"))
							localTxn.Insert(k, vv.MarshalTo(nil))
						}
					}

					accountStateRoot, _ := localTxn.Hash(st)
					account.Root = types.BytesToHash(accountStateRoot)
				}

				if obj.DirtyCode {
					// write code to memory object, never failed
					// if failed, can't alloc memory, it will panic
					err := st.SetCode(obj.CodeHash, obj.Code)
					if err != nil {
						return err
					}
				}

				vv := account.MarshalWith(arena)
				data := vv.MarshalTo(nil)

				tt.Insert(hashit(obj.Address.Bytes()), data)
				arena.Reset()
			}
		}

		var err error

		root, err = tt.Hash(st)
		if err != nil {
			return err
		}

		// dont use st here, we need to use the original stateDB
		nTrie = &Trie{
			stateDB: t.stateDB,
			root:    tt.root,
			epoch:   tt.epoch,
		}
		nTrie.stateDB = t.stateDB

		// Commit all the entries to db
		return st.Commit()
	})

	return nTrie, root, err
}

// Hash returns the root hash of the trie. It does not write to the
// database and can be used even if the trie doesn't have one.
func (t *Trie) Hash() types.Hash {
	if t.root == nil {
		return types.EmptyRootHash
	}

	hash, cached, _ := t.hashRoot()
	t.root = cached

	return types.BytesToHash(hash)
}

func (t *Trie) hashRoot() ([]byte, Node, error) {
	hash, _ := t.root.Hash()

	return hash, t.root, nil
}

func (t *Trie) Txn() *Txn {
	return &Txn{root: t.root, epoch: t.epoch + 1, reader: t.stateDB}
}

func prefixLen(k1, k2 []byte) int {
	max := len(k1)
	if l := len(k2); l < max {
		max = l
	}

	var i int

	for i = 0; i < max; i++ {
		if k1[i] != k2[i] {
			break
		}
	}

	return i
}

func concat(a, b []byte) []byte {
	c := make([]byte, len(a)+len(b))
	copy(c, a)
	copy(c[len(a):], b)

	return c
}

func extendByteSlice(b []byte, needLen int) []byte {
	b = b[:cap(b)]
	if n := needLen - cap(b); n > 0 {
		b = append(b, make([]byte, n)...)
	}

	return b[:needLen]
}
