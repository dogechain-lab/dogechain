package itrie

import (
	"github.com/dogechain-lab/dogechain/crypto"
	"github.com/dogechain-lab/dogechain/types"
)

type Trie struct {
	root  Node
	epoch uint32
}

func NewTrie() *Trie {
	return &Trie{}
}

func (t *Trie) Get(k []byte, reader StateDBReader) ([]byte, error) {
	txn := t.Txn(reader)

	return txn.Lookup(k)
}

func hashit(k []byte) []byte {
	return crypto.Keccak256(k)
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

// Txn returns a txn using current db instance
//
// Trie will not hold this db instance
func (t *Trie) Txn(reader StateDBReader) *Txn {
	return &Txn{reader: reader, root: t.root, epoch: t.epoch + 1}
}
