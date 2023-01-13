package itrie

import "sync"

const (
	batchAlloc  = 1024
	preBuffSize = 32
)

type preAllocPool struct {
	valueNodes []*ValueNode
	shortNodes []*ShortNode
	fullNodes  []*FullNode

	valueNodeMux sync.Mutex
	shortNodeMux sync.Mutex
	fullNodeMux  sync.Mutex
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
			valueNodes: make([]*ValueNode, 0, batchAlloc),
			shortNodes: make([]*ShortNode, 0, batchAlloc),
			fullNodes:  make([]*FullNode, 0, batchAlloc),
		},
	}
}

//nolint:dupl
func (np *NodePool) GetValueNode() *ValueNode {
	if node, ok := np.valueNodes.Get().(*ValueNode); ok && node != nil {
		return node
	}

	np.preAllocPool.valueNodeMux.Lock()
	defer np.preAllocPool.valueNodeMux.Unlock()

	if len(np.preAllocPool.valueNodes) > 0 {
		node := np.preAllocPool.valueNodes[len(np.preAllocPool.valueNodes)-1]
		np.preAllocPool.valueNodes = np.preAllocPool.valueNodes[:len(np.preAllocPool.valueNodes)-1]

		return node
	}

	// pre-allocate 1024 value node
	// clear pool and reset size
	np.preAllocPool.valueNodes = np.preAllocPool.valueNodes[0:batchAlloc]

	nodes := make([]ValueNode, batchAlloc)
	bufs := make([][preBuffSize]byte, batchAlloc)

	for i := 0; i < batchAlloc; i++ {
		nodes[i].buf = bufs[i][:0]
		np.preAllocPool.valueNodes[i] = &nodes[i]
	}

	// return last one
	node := np.preAllocPool.valueNodes[len(np.preAllocPool.valueNodes)-1]
	np.preAllocPool.valueNodes = np.preAllocPool.valueNodes[:len(np.preAllocPool.valueNodes)-1]

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

	np.preAllocPool.shortNodeMux.Lock()
	defer np.preAllocPool.shortNodeMux.Unlock()

	if len(np.preAllocPool.shortNodes) > 0 {
		node := np.preAllocPool.shortNodes[len(np.preAllocPool.shortNodes)-1]
		np.preAllocPool.shortNodes = np.preAllocPool.shortNodes[:len(np.preAllocPool.shortNodes)-1]

		return node
	}

	// pre-allocate 1024 value node
	// clear pool and reset size
	np.preAllocPool.shortNodes = np.preAllocPool.shortNodes[0:batchAlloc]

	nodes := make([]ShortNode, batchAlloc)
	hash := make([][preBuffSize]byte, batchAlloc)
	keys := make([][preBuffSize]byte, batchAlloc)

	for i := 0; i < batchAlloc; i++ {
		nodes[i].hash = hash[i][:0]
		nodes[i].key = keys[i][:0]

		np.preAllocPool.shortNodes[i] = &nodes[i]
	}

	// return last one
	node := np.preAllocPool.shortNodes[len(np.preAllocPool.shortNodes)-1]
	np.preAllocPool.shortNodes = np.preAllocPool.shortNodes[:len(np.preAllocPool.shortNodes)-1]

	return node
}

func (np *NodePool) PutShortNode(node *ShortNode) {
	node.key = node.key[0:0]
	node.hash = node.hash[0:0]

	np.PutNode(node.child)
	node.child = nil

	np.valueNodes.Put(node)
}

//nolint:dupl
func (np *NodePool) GetFullNode() *FullNode {
	if node, ok := np.fullNodes.Get().(*FullNode); ok && node != nil {
		return node
	}

	np.preAllocPool.fullNodeMux.Lock()
	defer np.preAllocPool.fullNodeMux.Unlock()

	if len(np.preAllocPool.fullNodes) > 0 {
		node := np.preAllocPool.fullNodes[len(np.preAllocPool.fullNodes)-1]
		np.preAllocPool.fullNodes = np.preAllocPool.fullNodes[:len(np.preAllocPool.fullNodes)-1]

		return node
	}

	// pre-allocate 1024 value node
	// clear pool and reset size
	np.preAllocPool.fullNodes = np.preAllocPool.fullNodes[0:batchAlloc]

	nodes := make([]FullNode, batchAlloc)
	hash := make([][preBuffSize]byte, batchAlloc)

	for i := 0; i < batchAlloc; i++ {
		nodes[i].hash = hash[i][:0]

		np.preAllocPool.fullNodes[i] = &nodes[i]
	}

	// return last one
	node := np.preAllocPool.fullNodes[len(np.preAllocPool.fullNodes)-1]
	np.preAllocPool.fullNodes = np.preAllocPool.fullNodes[:len(np.preAllocPool.fullNodes)-1]

	return node
}

func (np *NodePool) PutFullNode(node *FullNode) {
	node.hash = node.hash[0:0]
	node.epoch = 0

	np.PutNode(node.value)
	node.value = nil

	for i := 0; i < 16; i++ {
		np.PutNode(node.children[i])
		node.children[i] = nil
	}

	np.valueNodes.Put(node)
}

func (np *NodePool) PutNode(node Node) {
	switch n := node.(type) {
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
