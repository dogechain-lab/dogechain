package itrie

type Txn struct {
	reader StateDBReader

	root  Node
	epoch uint32
}

func (t *Txn) Lookup(key []byte) ([]byte, error) {
	node, res, err := lookupNode(t.reader, t.root, bytesToHexNibbles(key))
	nodePool.PutNode(node)

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
