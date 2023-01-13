package itrie

import (
	"encoding/binary"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Test that the node pool is working as expected.
// check get node from pool is new initialized

func TestNodePool_Get(t *testing.T) {
	np := NewNodePool()

	for i := 0; i < (batchAlloc * 2); i++ {
		{
			node := np.GetFullNode()

			assert.NotNil(t, node)
			assert.Nil(t, node.value)

			assert.Zero(t, len(node.hash))
			assert.NotZero(t, cap(node.hash))

			for j := 0; j < len(node.children); j++ {
				assert.Nil(t, node.children[j])

				// fill nil childrens reference
				node.children[j] = node
			}

			// fill nil value reference
			node.value = node
		}

		{
			node := np.GetShortNode()

			assert.NotNil(t, node)

			assert.Nil(t, node.child)

			assert.Zero(t, len(node.hash))
			assert.Zero(t, len(node.key))

			assert.NotZero(t, cap(node.hash))
			assert.NotZero(t, cap(node.key))

			// fill nil child reference
			node.child = node
			binary.BigEndian.PutUint64(node.hash[:8], uint64(i))
			binary.BigEndian.PutUint64(node.key[:8], uint64(i))
		}

		{
			node := np.GetValueNode()

			assert.NotNil(t, node)
			assert.Zero(t, len(node.buf))
			assert.NotZero(t, cap(node.buf))
			assert.False(t, node.hash)

			// fill object
			node.hash = true
			binary.BigEndian.PutUint64(node.buf[:8], uint64(i))
		}
	}
}

func TestNodePool_UniqueObject(t *testing.T) {
	np := NewNodePool()

	ptrMap := make(map[uintptr]interface{})
	ptrNoExist := func(obj interface{}) bool {
		ptr := reflect.ValueOf(obj).Pointer()

		_, exist := ptrMap[ptr]
		ptrMap[ptr] = obj

		return !exist
	}

	for i := 0; i < (batchAlloc * 2); i++ {
		{
			node := np.GetFullNode()

			assert.True(t, ptrNoExist(node))
			assert.True(t, ptrNoExist(node.hash))

			binary.BigEndian.PutUint64(node.hash[:8], uint64(i))

			for j := 0; j < len(node.children); j++ {
				assert.Nil(t, node.children[j])

				node.children[j] = node
			}
		}

		{
			node := np.GetShortNode()

			assert.True(t, ptrNoExist(node))
			assert.True(t, ptrNoExist(node.hash))
			assert.True(t, ptrNoExist(node.key))

			binary.BigEndian.PutUint64(node.hash[:8], uint64(i))
			binary.BigEndian.PutUint64(node.key[:8], uint64(i))

			node.child = node
		}

		{
			node := np.GetValueNode()

			assert.True(t, ptrNoExist(node))
			assert.True(t, ptrNoExist(node.buf))

			// fill object
			node.hash = true
			binary.BigEndian.PutUint64(node.buf[:8], uint64(i))
		}
	}
}

func TestTracerNodeTreeCircularReference(t *testing.T) {
	// long circular reference
	nodeList := make([]*ShortNode, 100)
	refMap := make(map[Node]bool)

	for i := 0; i < len(nodeList); i++ {
		nodeList[i] = nodePool.GetShortNode()
		refMap[nodeList[i]] = true
	}

	// circular reference
	for i := 0; i < len(nodeList); i++ {
		if i == len(nodeList)-1 {
			nodeList[i].child = nodeList[0]

			continue
		}

		nodeList[i].child = nodeList[i+1]
	}

	nodes := tracerNodeTree(nodeList[0])
	assert.Equal(t, len(nodeList), len(nodes))

	for i := 0; i < len(nodes); i++ {
		assert.True(t, refMap[nodes[i]])
	}

	// check allocation node number equal to trace node
	{
		// short -> full -> short -> value
		root := nodePool.GetShortNode()

		f1 := nodePool.GetFullNode()
		for i := 0; i < len(f1.children); i++ {
			s2 := nodePool.GetShortNode()
			s2.child = nodePool.GetValueNode()

			f1.children[i] = s2
		}

		root.child = f1

		assert.Equal(t, 34, len(tracerNodeTree(root)))
	}

	{
		// random full -> short -> value
		random := rand.New(rand.NewSource(time.Now().UnixNano()))

		root := nodePool.GetFullNode()
		allocCount := 1

		var fillFullNode func(node *FullNode, depth int)

		fillFullNode = func(node *FullNode, depth int) {
			if depth <= 0 {
				return
			}

			for i := 0; i < len(node.children); i++ {
				c := random.Int31n(3)

				switch c {
				case 0:
					child := nodePool.GetFullNode()
					fillFullNode(child, depth-1)

					node.children[i] = child
				case 1:
					node.children[i] = nodePool.GetShortNode()
				case 2:
					node.children[i] = nodePool.GetValueNode()
				}

				allocCount++
			}
		}

		fillFullNode(root, 4)

		assert.Equal(t, allocCount, len(tracerNodeTree(root)))
	}
}
