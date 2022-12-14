package itrie

type Txn struct {
	reader StateDBReader

	root  Node
	epoch uint32
}

func (t *Txn) Lookup(key []byte) []byte {
	_, res := lookupNode(t.reader, t.root, bytesToHexNibbles(key))

	return res
}

func (t *Txn) Insert(key, value []byte) {
	root := insertNode(t.reader, t.epoch, t.root, bytesToHexNibbles(key), value)
	if root != nil {
		t.root = root
	}
}

func (t *Txn) Delete(key []byte) {
	root, ok := deleteNode(t.reader, t.root, bytesToHexNibbles(key))
	if ok {
		t.root = root
	}
}
