package itrie

import (
	"encoding/binary"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test that the node pool is working as expected.
func TestNodePool_Get(t *testing.T) {
	np := NewNodePool()

	for i := 0; i < (nodePoolBatchAlloc * 2); i++ {
		{
			node := np.GetFullNode()

			assert.NotNil(t, node)
			assert.Nil(t, node.value)
			assert.Zero(t, len(node.hash))
			assert.NotZero(t, cap(node.hash))

			for j := 0; j < 16; j++ {
				assert.Nil(t, node.children[j])
			}
		}

		{
			node := np.GetShortNode()

			assert.NotNil(t, node)

			assert.Nil(t, node.child)

			assert.Zero(t, len(node.hash))
			assert.Zero(t, len(node.key))

			assert.NotZero(t, cap(node.hash))
			assert.NotZero(t, cap(node.key))
		}

		{
			node := np.GetValueNode()

			assert.NotNil(t, node)
			assert.Zero(t, len(node.buf))
			assert.NotZero(t, cap(node.buf))
			assert.False(t, node.hash)
		}
	}
}

func TestNodePool_UniqueObject(t *testing.T) {
	np := NewNodePool()

	objs := make([]interface{}, 0, nodePoolBatchAlloc*2)
	ptrMap := make(map[uintptr]bool)

	ptrNoExist := func(obj interface{}) bool {
		ptr := reflect.ValueOf(obj).Pointer()

		_, exist := ptrMap[ptr]
		ptrMap[ptr] = true

		return !exist
	}

	for i := 0; i < (nodePoolBatchAlloc * 2); i++ {
		{
			node := np.GetFullNode()

			assert.True(t, ptrNoExist(node))
			assert.True(t, ptrNoExist(node.hash))

			binary.BigEndian.PutUint64(node.hash[:8], uint64(i))

			objs = append(objs, node)
		}

		{
			node := np.GetShortNode()

			assert.True(t, ptrNoExist(node))
			assert.True(t, ptrNoExist(node.hash))
			assert.True(t, ptrNoExist(node.key))

			binary.BigEndian.PutUint64(node.hash[:8], uint64(i))
			binary.BigEndian.PutUint64(node.key[:8], uint64(i))

			objs = append(objs, node)
		}

		{
			node := np.GetValueNode()

			assert.True(t, ptrNoExist(node))
			assert.True(t, ptrNoExist(node.buf))

			binary.BigEndian.PutUint64(node.buf[:8], uint64(i))

			objs = append(objs, node)
		}
	}

	assert.True(t, len(objs) == (nodePoolBatchAlloc*2*3))
}
