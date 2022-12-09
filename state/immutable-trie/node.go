package itrie

import (
	"bytes"
	"fmt"
)

// Node represents a node reference
type Node interface {
	Hash() ([]byte, bool)
	SetHash(b []byte) []byte
}

// ValueNode is a leaf on the merkle-trie
type ValueNode struct {
	// hash marks if this value node represents a stored node
	hash bool
	buf  []byte
}

// Hash implements the node interface
func (v *ValueNode) Hash() ([]byte, bool) {
	return v.buf, v.hash
}

// SetHash implements the node interface
func (v *ValueNode) SetHash(b []byte) []byte {
	panic("We cannot set hash on value node")
}

type common struct {
	hash []byte
}

// Hash implements the node interface
func (c *common) Hash() ([]byte, bool) {
	return c.hash, len(c.hash) != 0
}

// SetHash implements the node interface
func (c *common) SetHash(b []byte) []byte {
	c.hash = extendByteSlice(c.hash, len(b))
	copy(c.hash, b)

	return c.hash
}

// ShortNode is an extension or short node
type ShortNode struct {
	common
	key   []byte
	child Node
}

// FullNode is a node with several children
type FullNode struct {
	common
	epoch    uint32
	value    Node
	children [16]Node
}

func (f *FullNode) copy() *FullNode {
	nc := &FullNode{}
	nc.value = f.value
	copy(nc.children[:], f.children[:])

	return nc
}

func (f *FullNode) setEdge(idx byte, e Node) {
	if idx == 16 {
		f.value = e
	} else {
		f.children[idx] = e
	}
}

func (f *FullNode) getEdge(idx byte) Node {
	if idx == 16 {
		return f.value
	} else {
		return f.children[idx]
	}
}

func lookupNode(storage StorageReader, node interface{}, key []byte) (Node, []byte) {
	switch n := node.(type) {
	case nil:
		return nil, nil

	case *ValueNode:
		if n.hash {
			nc, ok, err := GetNode(n.buf, storage)
			if err != nil {
				panic(err)
			}

			if !ok {
				return nil, nil
			}

			_, res := lookupNode(storage, nc, key)

			return nc, res
		}

		if len(key) == 0 {
			return nil, n.buf
		} else {
			return nil, nil
		}

	case *ShortNode:
		plen := len(n.key)
		if plen > len(key) || !bytes.Equal(key[:plen], n.key) {
			return nil, nil
		}

		child, res := lookupNode(storage, n.child, key[plen:])

		if child != nil {
			n.child = child
		}

		return nil, res

	case *FullNode:
		if len(key) == 0 {
			return lookupNode(storage, n.value, key)
		}

		child, res := lookupNode(storage, n.getEdge(key[0]), key[1:])

		if child != nil {
			n.children[key[0]] = child
		}

		return nil, res

	default:
		panic(fmt.Sprintf("unknown node type %v", n))
	}
}

func insertNode(storage StorageReader, epoch uint32, node Node, search, value []byte) Node {
	switch n := node.(type) {
	case nil:
		// NOTE, this only happens with the full node
		if len(search) == 0 {
			v := &ValueNode{}
			v.buf = make([]byte, len(value))
			copy(v.buf, value)

			return v
		} else {
			return &ShortNode{
				key:   search,
				child: insertNode(storage, epoch, nil, nil, value),
			}
		}

	case *ValueNode:
		if n.hash {
			nc, ok, err := GetNode(n.buf, storage)
			if err != nil {
				panic(err)
			}

			if !ok {
				return nil
			}

			return insertNode(storage, epoch, nc, search, value)
		}

		if len(search) == 0 {
			v := &ValueNode{}
			v.buf = make([]byte, len(value))
			copy(v.buf, value)

			return v
		} else {
			b := insertNode(storage, epoch, &FullNode{epoch: epoch, value: n}, search, value)

			return b
		}

	case *ShortNode:
		plen := prefixLen(search, n.key)
		if plen == len(n.key) {
			// Keep this node as is and insert to child
			child := insertNode(storage, epoch, n.child, search[plen:], value)

			return &ShortNode{key: n.key, child: child}
		} else {
			// Introduce a new branch
			b := FullNode{epoch: epoch}
			if len(n.key) > plen+1 {
				b.setEdge(n.key[plen], &ShortNode{key: n.key[plen+1:], child: n.child})
			} else {
				b.setEdge(n.key[plen], n.child)
			}

			child := insertNode(storage, epoch, &b, search[plen:], value)

			if plen == 0 {
				return child
			} else {
				return &ShortNode{key: search[:plen], child: child}
			}
		}

	case *FullNode:
		// b := t.writeNode(n)
		nc := n
		if epoch != n.epoch {
			nc = &FullNode{
				epoch: epoch,
				value: n.value,
			}
			copy(nc.children[:], n.children[:])
		}

		if len(search) == 0 {
			nc.value = insertNode(storage, epoch, nc.value, nil, value)

			return nc
		} else {
			k := search[0]
			child := n.getEdge(k)
			newChild := insertNode(storage, epoch, child, search[1:], value)
			if child == nil {
				nc.setEdge(k, newChild)
			} else {
				nc.setEdge(k, newChild)
			}

			return nc
		}

	default:
		panic(fmt.Sprintf("unknown node type %v", n))
	}
}

func deleteNode(storage StorageReader, node Node, search []byte) (Node, bool) {
	switch n := node.(type) {
	case nil:
		return nil, false

	case *ShortNode:
		n.hash = n.hash[:0]

		plen := prefixLen(search, n.key)
		if plen == len(search) {
			return nil, true
		}

		if plen == 0 {
			return nil, false
		}

		child, ok := deleteNode(storage, n.child, search[plen:])
		if !ok {
			return nil, false
		}

		if child == nil {
			return nil, true
		}

		if short, ok := child.(*ShortNode); ok {
			// merge nodes
			return &ShortNode{key: concat(n.key, short.key), child: short.child}, true
		} else {
			// full node
			return &ShortNode{key: n.key, child: child}, true
		}

	case *ValueNode:
		if n.hash {
			nc, ok, err := GetNode(n.buf, storage)
			if err != nil {
				panic(err)
			}

			if !ok {
				return nil, false
			}

			return deleteNode(storage, nc, search)
		}

		if len(search) != 0 {
			return nil, false
		}

		return nil, true

	case *FullNode:
		n = n.copy()
		n.hash = n.hash[:0]

		key := search[0]
		newChild, ok := deleteNode(storage, n.getEdge(key), search[1:])

		if !ok {
			return nil, false
		}

		n.setEdge(key, newChild)

		indx := -1

		var notEmpty bool

		for edge, i := range n.children {
			if i != nil {
				if indx != -1 {
					notEmpty = true

					break
				} else {
					indx = edge
				}
			}
		}

		if indx != -1 && n.value != nil {
			// We have one children and value, set notEmpty to true
			notEmpty = true
		}

		if notEmpty {
			// The full node still has some other values
			return n, true
		}

		if indx == -1 {
			// There are no children nodes
			if n.value == nil {
				// Everything is empty, return nil
				return nil, true
			}
			// The value is the only left, return a short node with it
			return &ShortNode{key: []byte{0x10}, child: n.value}, true
		}

		// Only one value left at indx
		nc := n.children[indx]

		if vv, ok := nc.(*ValueNode); ok && vv.hash {
			// If the value is a hash, we have to resolve it first.
			// This needs better testing
			aux, ok, err := GetNode(vv.buf, storage)
			if err != nil {
				panic(err)
			}

			if !ok {
				return nil, false
			}

			nc = aux
		}

		obj, ok := nc.(*ShortNode)
		if !ok {
			obj := &ShortNode{}
			obj.key = []byte{byte(indx)}
			obj.child = nc

			return obj, true
		}

		ncc := &ShortNode{}
		ncc.key = concat([]byte{byte(indx)}, obj.key)
		ncc.child = obj.child

		return ncc, true
	}

	panic("it should not happen")
}
