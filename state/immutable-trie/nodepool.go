package itrie

import "sync"

const (
	nodePoolBatchAlloc    = 1024
	nodeBufferPreAllocLen = 32
)

type NodePool struct {
	valueNodePool sync.Pool
	shortNodePool sync.Pool
	fullNodePool  sync.Pool

	valueNodePreAllocPool []*ValueNode
	shortNodePreAllocPool []*ShortNode
	fullNodePreAllocPool  []*FullNode

	valueNodePreAllocMux sync.Mutex
	shortNodePreAllocMux sync.Mutex
	fullNodePreAllocMux  sync.Mutex
}

func NewNodePool() *NodePool {
	return &NodePool{
		valueNodePreAllocPool: make([]*ValueNode, 0, nodePoolBatchAlloc),
		shortNodePreAllocPool: make([]*ShortNode, 0, nodePoolBatchAlloc),
		fullNodePreAllocPool:  make([]*FullNode, 0, nodePoolBatchAlloc),
	}
}

//nolint:dupl
func (np *NodePool) GetValueNode() *ValueNode {
	if node, ok := np.valueNodePool.Get().(*ValueNode); ok && node != nil {
		return node
	}

	np.valueNodePreAllocMux.Lock()
	defer np.valueNodePreAllocMux.Unlock()

	if len(np.valueNodePreAllocPool) > 0 {
		node := np.valueNodePreAllocPool[len(np.valueNodePreAllocPool)-1]
		np.valueNodePreAllocPool = np.valueNodePreAllocPool[:len(np.valueNodePreAllocPool)-1]

		return node
	}

	// pre-allocate 1024 value node
	// clear pool and reset size
	np.valueNodePreAllocPool = np.valueNodePreAllocPool[0:nodePoolBatchAlloc]

	nodes := make([]ValueNode, nodePoolBatchAlloc)
	bufs := make([][nodeBufferPreAllocLen]byte, nodePoolBatchAlloc)

	for i := 0; i < nodePoolBatchAlloc; i++ {
		nodes[i].buf = bufs[i][:0]
		np.valueNodePreAllocPool[i] = &nodes[i]
	}

	// return last one
	node := np.valueNodePreAllocPool[len(np.valueNodePreAllocPool)-1]
	np.valueNodePreAllocPool = np.valueNodePreAllocPool[:len(np.valueNodePreAllocPool)-1]

	return node
}

func (np *NodePool) PutValueNode(node *ValueNode) {
	node.buf = node.buf[0:0]
	node.hash = false

	np.valueNodePool.Put(node)
}

func (np *NodePool) GetShortNode() *ShortNode {
	if node, ok := np.shortNodePool.Get().(*ShortNode); ok && node != nil {
		return node
	}

	np.shortNodePreAllocMux.Lock()
	defer np.shortNodePreAllocMux.Unlock()

	if len(np.shortNodePreAllocPool) > 0 {
		node := np.shortNodePreAllocPool[len(np.shortNodePreAllocPool)-1]
		np.shortNodePreAllocPool = np.shortNodePreAllocPool[:len(np.shortNodePreAllocPool)-1]

		return node
	}

	// pre-allocate 1024 value node
	// clear pool and reset size
	np.shortNodePreAllocPool = np.shortNodePreAllocPool[0:nodePoolBatchAlloc]

	nodes := make([]ShortNode, nodePoolBatchAlloc)
	hash := make([][nodeBufferPreAllocLen]byte, nodePoolBatchAlloc)
	keys := make([][nodeBufferPreAllocLen]byte, nodePoolBatchAlloc)

	for i := 0; i < nodePoolBatchAlloc; i++ {
		nodes[i].hash = hash[i][:0]
		nodes[i].key = keys[i][:0]

		np.shortNodePreAllocPool[i] = &nodes[i]
	}

	// return last one
	node := np.shortNodePreAllocPool[len(np.shortNodePreAllocPool)-1]
	np.shortNodePreAllocPool = np.shortNodePreAllocPool[:len(np.shortNodePreAllocPool)-1]

	return node
}

func (np *NodePool) PutShortNode(node *ShortNode) {
	node.key = node.key[0:0]
	node.hash = node.hash[0:0]

	np.PutNode(node.child)
	node.child = nil

	np.valueNodePool.Put(node)
}

//nolint:dupl
func (np *NodePool) GetFullNode() *FullNode {
	if node, ok := np.fullNodePool.Get().(*FullNode); ok && node != nil {
		return node
	}

	np.fullNodePreAllocMux.Lock()
	defer np.fullNodePreAllocMux.Unlock()

	if len(np.fullNodePreAllocPool) > 0 {
		node := np.fullNodePreAllocPool[len(np.fullNodePreAllocPool)-1]
		np.fullNodePreAllocPool = np.fullNodePreAllocPool[:len(np.fullNodePreAllocPool)-1]

		return node
	}

	// pre-allocate 1024 value node
	// clear pool and reset size
	np.fullNodePreAllocPool = np.fullNodePreAllocPool[0:nodePoolBatchAlloc]

	nodes := make([]FullNode, nodePoolBatchAlloc)
	hash := make([][nodeBufferPreAllocLen]byte, nodePoolBatchAlloc)

	for i := 0; i < nodePoolBatchAlloc; i++ {
		nodes[i].hash = hash[i][:0]

		np.fullNodePreAllocPool[i] = &nodes[i]
	}

	// return last one
	node := np.fullNodePreAllocPool[len(np.fullNodePreAllocPool)-1]
	np.fullNodePreAllocPool = np.fullNodePreAllocPool[:len(np.fullNodePreAllocPool)-1]

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

	np.valueNodePool.Put(node)
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
