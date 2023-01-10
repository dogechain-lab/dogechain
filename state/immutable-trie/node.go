package itrie

import (
	"bytes"
	"fmt"
)

var nodePool = NewNodePool()

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
	nc := nodePool.GetFullNode()
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

func lookupNode(storage StorageReader, node interface{}, key []byte) (Node, []byte, error) {
	switch n := node.(type) {
	case nil:
		return nil, nil, nil

	case *ValueNode:
		if n.hash {
			nc, ok, err := GetNode(n.buf, storage)
			if err != nil {
				return nil, nil, err
			}

			if !ok {
				return nil, nil, nil
			}

			_, res, err := lookupNode(storage, nc, key)

			return nc, res, err
		}

		if len(key) == 0 {
			return nil, n.buf, nil
		} else {
			return nil, nil, nil
		}

	case *ShortNode:
		plen := len(n.key)
		if plen > len(key) || !bytes.Equal(key[:plen], n.key) {
			return nil, nil, nil
		}

		child, res, err := lookupNode(storage, n.child, key[plen:])

		if child != nil {
			n.child = child
		}

		return nil, res, err

	case *FullNode:
		if len(key) == 0 {
			return lookupNode(storage, n.value, key)
		}

		child, res, err := lookupNode(storage, n.getEdge(key[0]), key[1:])

		if child != nil {
			n.children[key[0]] = child
		}

		return nil, res, err

	default:
		panic(fmt.Sprintf("unknown node type %v", n))
	}
}

func insertNode(storage StorageReader, epoch uint32, node Node, search, value []byte) (Node, error) {
	switch n := node.(type) {
	case nil:
		// NOTE, this only happens with the full node
		if len(search) == 0 {
			v := nodePool.GetValueNode()
			v.buf = append(v.buf[0:0], value...)

			return v, nil
		} else {
			child, err := insertNode(storage, epoch, nil, nil, value)

			sn := nodePool.GetShortNode()
			sn.key = append(sn.key[0:0], search...)
			sn.child = child

			return sn, err
		}

	case *ValueNode:
		if n.hash {
			nc, ok, err := GetNode(n.buf, storage)
			if err != nil {
				return nil, err
			}

			if !ok {
				return nil, nil
			}

			return insertNode(storage, epoch, nc, search, value)
		}

		if len(search) == 0 {
			v := nodePool.GetValueNode()
			v.buf = append(v.buf[0:0], value...)

			return v, nil
		} else {
			fn := nodePool.GetFullNode()
			fn.epoch = epoch
			fn.value = n

			return insertNode(storage, epoch, fn, search, value)
		}

	case *ShortNode:
		plen := prefixLen(search, n.key)
		if plen == len(n.key) {
			// Keep this node as is and insert to child
			child, err := insertNode(storage, epoch, n.child, search[plen:], value)

			sn := nodePool.GetShortNode()
			sn.key = append(sn.key[0:0], n.key...)
			sn.child = child

			return sn, err
		} else {
			// Introduce a new branch
			b := nodePool.GetFullNode()
			b.epoch = epoch

			if len(n.key) > plen+1 {
				sn := nodePool.GetShortNode()
				sn.key = append(sn.key[0:0], n.key[plen+1:]...)
				sn.child = n.child

				b.setEdge(n.key[plen], sn)
			} else {
				b.setEdge(n.key[plen], n.child)
			}

			child, err := insertNode(storage, epoch, b, search[plen:], value)

			if plen == 0 {
				return child, err
			} else {
				sn := nodePool.GetShortNode()
				sn.key = append(sn.key[0:0], search[:plen]...)
				sn.child = child

				return sn, err
			}
		}

	case *FullNode:
		nc := n
		if epoch != n.epoch {
			nc = nodePool.GetFullNode()
			nc.epoch = epoch
			nc.value = n.value
			copy(nc.children[:], n.children[:])
		}

		if len(search) == 0 {
			var err error
			nc.value, err = insertNode(storage, epoch, nc.value, nil, value)

			return nc, err
		} else {
			k := search[0]
			child := n.getEdge(k)
			newChild, err := insertNode(storage, epoch, child, search[1:], value)
			if child == nil {
				nc.setEdge(k, newChild)
			} else {
				nc.setEdge(k, newChild)
			}

			return nc, err
		}

	default:
		panic(fmt.Sprintf("unknown node type %v", n))
	}
}

func deleteNode(storage StorageReader, node Node, search []byte) (Node, bool, error) {
	switch n := node.(type) {
	case nil:
		return nil, false, nil

	case *ShortNode:
		n.hash = n.hash[:0]

		plen := prefixLen(search, n.key)
		if plen == len(search) {
			return nil, true, nil
		}

		if plen == 0 {
			return nil, false, nil
		}

		child, ok, err := deleteNode(storage, n.child, search[plen:])
		if err != nil {
			return nil, false, err
		}

		if !ok {
			return nil, false, nil
		}

		if child == nil {
			return nil, true, nil
		}

		if short, ok := child.(*ShortNode); ok {
			// merge nodes
			sn := nodePool.GetShortNode()
			sn.key = append(sn.key[0:0], concat(n.key, short.key)...)
			sn.child = short.child

			return sn, true, nil
		} else {
			// full node
			sn := nodePool.GetShortNode()
			sn.key = append(sn.key[0:0], n.key...)
			sn.child = child

			return sn, true, nil
		}

	case *ValueNode:
		if n.hash {
			nc, ok, err := GetNode(n.buf, storage)
			if err != nil {
				return nil, false, err
			}

			if !ok {
				return nil, false, nil
			}

			return deleteNode(storage, nc, search)
		}

		if len(search) != 0 {
			return nil, false, nil
		}

		return nil, true, nil

	case *FullNode:
		n = n.copy()
		n.hash = n.hash[:0]

		key := search[0]

		newChild, ok, err := deleteNode(storage, n.getEdge(key), search[1:])
		if err != nil {
			return nil, false, err
		}

		if !ok {
			return nil, false, err
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
			return n, true, nil
		}

		if indx == -1 {
			// There are no children nodes
			if n.value == nil {
				// Everything is empty, return nil
				return nil, true, nil
			}
			// The value is the only left, return a short node with it
			sn := nodePool.GetShortNode()
			sn.key = append(sn.key[0:0], 0x10)
			sn.child = n.value

			return sn, true, nil
		}

		// Only one value left at indx
		nc := n.children[indx]

		if vv, ok := nc.(*ValueNode); ok && vv.hash {
			// If the value is a hash, we have to resolve it first.
			// This needs better testing
			aux, ok, err := GetNode(vv.buf, storage)
			if err != nil {
				return nil, false, err
			}

			if !ok {
				return nil, false, nil
			}

			nc = aux
		}

		obj, ok := nc.(*ShortNode)
		if !ok {
			obj := nodePool.GetShortNode()
			obj.key = append(obj.key[0:0], []byte{byte(indx)}...)
			obj.child = nc

			return obj, true, nil
		}

		ncc := nodePool.GetShortNode()

		ncc.key = append(obj.key[0:0], concat([]byte{byte(indx)}, obj.key)...)
		ncc.child = obj.child

		return ncc, true, nil
	}

	panic("it should not happen")
}
