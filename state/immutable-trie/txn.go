package itrie

type Txn struct {
	reader StateDBReader

	root  Node
	epoch uint32
}

// Commit returns a committed Trie with the root
func (t *Txn) Commit() *Trie {
	return &Trie{epoch: t.epoch, root: t.root}
}

func (t *Txn) Lookup(key []byte) ([]byte, error) {
	_, res, err := lookupNode(t.reader, t.root, bytesToHexNibbles(key))

	return res, err
}

func (t *Txn) Insert(key, value []byte) error {
	root, err := insertNode(t.reader, t.epoch, t.root, bytesToHexNibbles(key), value)

	if err != nil {
		return err
	}

	if root != nil {
		t.root = root
	}

	return nil
}

func (t *Txn) Delete(key []byte) error {
	root, ok, err := deleteNode(t.reader, t.root, bytesToHexNibbles(key))
	if err != nil {
		return err
	}

	if ok {
		t.root = root
	}

	return nil
}
