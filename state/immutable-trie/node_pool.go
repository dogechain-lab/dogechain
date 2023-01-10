package itrie

import "sync"

const (
	nodePoolBatchAlloc = 1024
)

type NodePool struct {
	valueNodePool []*ValueNode
	shortNodePool []*ShortNode
	fullNodePool  []*FullNode

	valueNodePreAllocMux sync.Mutex
	shortNodePreAllocMux sync.Mutex
	fullNodePreAllocMux  sync.Mutex
}

func NewNodePool() *NodePool {
	return &NodePool{
		valueNodePool: make([]*ValueNode, 0, nodePoolBatchAlloc),
		shortNodePool: make([]*ShortNode, 0, nodePoolBatchAlloc),
		fullNodePool:  make([]*FullNode, 0, nodePoolBatchAlloc),
	}
}

//nolint:dupl
func (np *NodePool) GetValueNode() *ValueNode {
	np.valueNodePreAllocMux.Lock()
	defer np.valueNodePreAllocMux.Unlock()

	if len(np.valueNodePool) > 0 {
		node := np.valueNodePool[len(np.valueNodePool)-1]
		np.valueNodePool = np.valueNodePool[:len(np.valueNodePool)-1]

		return node
	}

	// pre-allocate 1024 value node
	// clear pool and reset size
	np.valueNodePool = np.valueNodePool[0:nodePoolBatchAlloc]

	nodes := make([]ValueNode, nodePoolBatchAlloc)
	bufs := make([][32]byte, nodePoolBatchAlloc)

	for i := 0; i < nodePoolBatchAlloc; i++ {
		nodes[i].buf = bufs[i][:0]
		np.valueNodePool[i] = &nodes[i]
	}

	// return last one
	node := np.valueNodePool[len(np.valueNodePool)-1]
	np.valueNodePool = np.valueNodePool[:len(np.valueNodePool)-1]

	return node
}

func (np *NodePool) GetShortNode() *ShortNode {
	np.shortNodePreAllocMux.Lock()
	defer np.shortNodePreAllocMux.Unlock()

	if len(np.shortNodePool) > 0 {
		node := np.shortNodePool[len(np.shortNodePool)-1]
		np.shortNodePool = np.shortNodePool[:len(np.shortNodePool)-1]

		return node
	}

	// pre-allocate 1024 value node
	// clear pool and reset size
	np.shortNodePool = np.shortNodePool[0:nodePoolBatchAlloc]

	nodes := make([]ShortNode, nodePoolBatchAlloc)
	hash := make([][32]byte, nodePoolBatchAlloc)
	keys := make([][32]byte, nodePoolBatchAlloc)

	for i := 0; i < nodePoolBatchAlloc; i++ {
		nodes[i].hash = hash[i][:0]
		nodes[i].key = keys[i][:0]

		np.shortNodePool[i] = &nodes[i]
	}

	// return last one
	node := np.shortNodePool[len(np.shortNodePool)-1]
	np.shortNodePool = np.shortNodePool[:len(np.shortNodePool)-1]

	return node
}

//nolint:dupl
func (np *NodePool) GetFullNode() *FullNode {
	np.fullNodePreAllocMux.Lock()
	defer np.fullNodePreAllocMux.Unlock()

	if len(np.fullNodePool) > 0 {
		node := np.fullNodePool[len(np.fullNodePool)-1]
		np.fullNodePool = np.fullNodePool[:len(np.fullNodePool)-1]

		return node
	}

	// pre-allocate 1024 value node
	// clear pool and reset size
	np.fullNodePool = np.fullNodePool[0:nodePoolBatchAlloc]

	nodes := make([]FullNode, nodePoolBatchAlloc)
	hash := make([][32]byte, nodePoolBatchAlloc)

	for i := 0; i < nodePoolBatchAlloc; i++ {
		nodes[i].hash = hash[i][:0]

		np.fullNodePool[i] = &nodes[i]
	}

	// return last one
	node := np.fullNodePool[len(np.fullNodePool)-1]
	np.fullNodePool = np.fullNodePool[:len(np.fullNodePool)-1]

	return node
}
