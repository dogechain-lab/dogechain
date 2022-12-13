package itrie

type Txn struct {
	root  Node
	epoch uint32
}

func (t *Txn) Commit() *Trie {
	return &Trie{epoch: t.epoch, root: t.root}
}

func (t *Txn) Lookup(state StateDB, key []byte) []byte {
	_, res := lookupNode(state, t.root, bytesToHexNibbles(key))

	return res
}

func (t *Txn) Insert(state StateDB, key, value []byte) {
	root := insertNode(state, t.epoch, t.root, bytesToHexNibbles(key), value)
	if root != nil {
		t.root = root
	}
}

func (t *Txn) Delete(state StateDB, key []byte) {
	root, ok := deleteNode(state, t.root, bytesToHexNibbles(key))
	if ok {
		t.root = root
	}
}
