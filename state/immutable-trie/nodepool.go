package itrie

import (
	"sync"
)

const (
	batchAlloc  = 1024
	preBuffSize = 32
)

type preAllocPool struct {
	values []*ValueNode
	shorts []*ShortNode
	fulls  []*FullNode

	valuesMux sync.Mutex
	shortsMux sync.Mutex
	fullsMux  sync.Mutex
}

type NodePool struct {
	preAllocPool

	valueNodes sync.Pool
	shortNodes sync.Pool
	fullNodes  sync.Pool
}

func NewNodePool() *NodePool {
	return &NodePool{
		preAllocPool: preAllocPool{
			values: make([]*ValueNode, 0, batchAlloc),
			shorts: make([]*ShortNode, 0, batchAlloc),
			fulls:  make([]*FullNode, 0, batchAlloc),
		},
	}
}

//nolint:dupl
func (np *NodePool) GetValueNode() *ValueNode {
	if node, ok := np.valueNodes.Get().(*ValueNode); ok && node != nil {
		return node
	}

	np.preAllocPool.valuesMux.Lock()
	defer np.preAllocPool.valuesMux.Unlock()

	if len(np.preAllocPool.values) > 0 {
		node := np.preAllocPool.values[len(np.preAllocPool.values)-1]
		np.preAllocPool.values = np.preAllocPool.values[:len(np.preAllocPool.values)-1]

		return node
	}

	// pre-allocate 1024 value node
	// clear pool and reset size
	np.preAllocPool.values = np.preAllocPool.values[0:batchAlloc]

	nodes := make([]ValueNode, batchAlloc)
	bufs := make([][preBuffSize]byte, batchAlloc)

	for i := 0; i < batchAlloc; i++ {
		nodes[i].buf = bufs[i][:0]
		np.preAllocPool.values[i] = &nodes[i]
	}

	// return last one
	node := np.preAllocPool.values[len(np.preAllocPool.values)-1]
	np.preAllocPool.values = np.preAllocPool.values[:len(np.preAllocPool.values)-1]

	return node
}

func (np *NodePool) PutValueNode(node *ValueNode) {
	node.buf = node.buf[0:0]
	node.hash = false

	np.valueNodes.Put(node)
}

func (np *NodePool) GetShortNode() *ShortNode {
	if node, ok := np.shortNodes.Get().(*ShortNode); ok && node != nil {
		return node
	}

	np.preAllocPool.shortsMux.Lock()
	defer np.preAllocPool.shortsMux.Unlock()

	if len(np.preAllocPool.shorts) > 0 {
		node := np.preAllocPool.shorts[len(np.preAllocPool.shorts)-1]
		np.preAllocPool.shorts = np.preAllocPool.shorts[:len(np.preAllocPool.shorts)-1]

		return node
	}

	// pre-allocate 1024 value node
	// clear pool and reset size
	np.preAllocPool.shorts = np.preAllocPool.shorts[0:batchAlloc]

	nodes := make([]ShortNode, batchAlloc)
	hash := make([][preBuffSize]byte, batchAlloc)
	keys := make([][preBuffSize]byte, batchAlloc)

	for i := 0; i < batchAlloc; i++ {
		nodes[i].hash = hash[i][:0]
		nodes[i].key = keys[i][:0]

		np.preAllocPool.shorts[i] = &nodes[i]
	}

	// return last one
	node := np.preAllocPool.shorts[len(np.preAllocPool.shorts)-1]
	np.preAllocPool.shorts = np.preAllocPool.shorts[:len(np.preAllocPool.shorts)-1]

	return node
}

func (np *NodePool) PutShortNode(node *ShortNode) {
	node.key = node.key[0:0]
	node.hash = node.hash[0:0]
	node.child = nil

	np.shortNodes.Put(node)
}

//nolint:dupl
func (np *NodePool) GetFullNode() *FullNode {
	if node, ok := np.fullNodes.Get().(*FullNode); ok && node != nil {
		return node
	}

	np.preAllocPool.fullsMux.Lock()
	defer np.preAllocPool.fullsMux.Unlock()

	if len(np.preAllocPool.fulls) > 0 {
		node := np.preAllocPool.fulls[len(np.preAllocPool.fulls)-1]
		np.preAllocPool.fulls = np.preAllocPool.fulls[:len(np.preAllocPool.fulls)-1]

		return node
	}

	// pre-allocate 1024 value node
	// clear pool and reset size
	np.preAllocPool.fulls = np.preAllocPool.fulls[0:batchAlloc]

	nodes := make([]FullNode, batchAlloc)
	hash := make([][preBuffSize]byte, batchAlloc)

	for i := 0; i < batchAlloc; i++ {
		nodes[i].hash = hash[i][:0]

		np.preAllocPool.fulls[i] = &nodes[i]
	}

	// return last one
	node := np.preAllocPool.fulls[len(np.preAllocPool.fulls)-1]
	np.preAllocPool.fulls = np.preAllocPool.fulls[:len(np.preAllocPool.fulls)-1]

	return node
}

func (np *NodePool) PutFullNode(node *FullNode) {
	node.hash = node.hash[0:0]
	node.epoch = 0
	node.value = nil

	for i := 0; i < 16; i++ {
		node.children[i] = nil
	}

	np.fullNodes.Put(node)
}

func tracerNodeTree(node Node) []Node {
	if node == nil {
		return []Node{}
	}

	// traverse the node tree (DFS)
	stack := make([]Node, 0, 64)
	stack = append(stack, node)

	// circular reference check
	refMap := make(map[interface{}]bool)
	existRefMap := func(n Node) bool {
		_, ok := refMap[n]

		return ok
	}

	stackIndex := 0

	for {
		if stackIndex >= len(stack) {
			break
		}

		n := stack[stackIndex]
		if existRefMap(n) {
			// next node
			stackIndex++

			continue
		}

		refMap[n] = true

		switch n := n.(type) {
		case *ShortNode:
			// if node is not nil and not exist in refMap, add to stack
			if n.child != nil && !existRefMap(n.child) {
				stack = append(stack, n.child)
			}
		case *FullNode:
			for i := 0; i < len(n.children); i++ {
				if n.children[i] != nil && !existRefMap(n.children[i]) {
					stack = append(stack, n.children[i])
				}
			}

			if n.value != nil && !existRefMap(n.value) {
				stack = append(stack, n.value)
			}
		default:
		}

		stackIndex++
	}

	return stack
}

func (np *NodePool) PutNode(node Node) {
	if node == nil {
		return
	}

	for _, tn := range tracerNodeTree(node) {
		switch n := tn.(type) {
		case nil:
			return
		case *ValueNode:
			np.PutValueNode(n)
		case *ShortNode:
			np.PutShortNode(n)
		case *FullNode:
			np.PutFullNode(n)
		}
	}
}
