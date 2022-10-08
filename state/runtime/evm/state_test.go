package evm

import (
	"math/big"
	"testing"

	"github.com/dogechain-lab/dogechain/chain"
	"github.com/stretchr/testify/assert"
)

type codeHelper struct {
	buf []byte
}

func (c *codeHelper) Code() []byte {
	return c.buf
}

func (c *codeHelper) push1() {
	c.buf = append(c.buf, PUSH1)
	c.buf = append(c.buf, 0x1)
}

func (c *codeHelper) pop() {
	c.buf = append(c.buf, POP)
}

func getState() (*state, func()) {
	c := statePool.Get().(*state) //nolint:forcetypeassert

	return c, func() {
		c.reset()
		statePool.Put(c)
	}
}

func TestStackTop(t *testing.T) {
	s, closeFn := getState()
	defer closeFn()

	s.push(one)
	s.push(two)

	assert.Equal(t, two, s.top())
	assert.Equal(t, s.stackSize(), 2)
}

func TestStackOverflow(t *testing.T) {
	code := codeHelper{}
	for i := 0; i < stackSize; i++ {
		code.push1()
	}

	s, closeFn := getState()
	defer closeFn()

	s.code = code.buf
	s.gas = 10000

	_, err := s.Run()
	assert.NoError(t, err)

	// add one more item to the stack
	code.push1()

	s.reset()
	s.code = code.buf
	s.gas = 10000

	_, err = s.Run()
	assert.Equal(t, errStackOverflow, err)
}

func TestStackUnderflow(t *testing.T) {
	s, closeFn := getState()
	defer closeFn()

	code := codeHelper{}
	for i := 0; i < 10; i++ {
		code.push1()
	}

	for i := 0; i < 10; i++ {
		code.pop()
	}

	s.code = code.buf
	s.gas = 10000

	_, err := s.Run()
	assert.NoError(t, err)

	code.pop()

	s.reset()
	s.code = code.buf
	s.gas = 10000

	_, err = s.Run()
	assert.Equal(t, errStackUnderflow, err)
}

func TestOpcodeNotFound(t *testing.T) {
	s, closeFn := getState()
	defer closeFn()

	s.code = []byte{0xA5}
	s.gas = 1000

	_, err := s.Run()
	assert.Equal(t, errOpCodeNotFound, err)
}

func Test_extendMemory(t *testing.T) {
	testCases := []struct {
		name                   string
		memoryOffset           int64
		dataOffset             int64
		dataLength             int64
		expectedLengthBefore   int
		expectedCapacityBefore int
		expectedLengthAfter    int
		expectedCapacityAfter  int
	}{
		{
			name:                   "no need to extend memory",
			memoryOffset:           10,
			dataOffset:             0,
			dataLength:             16,
			expectedLengthBefore:   32,
			expectedCapacityBefore: 32,
			expectedLengthAfter:    32,
			expectedCapacityAfter:  32,
		},
		{
			name:                   "need to extend memory",
			memoryOffset:           32,
			dataOffset:             0,
			dataLength:             1,
			expectedLengthBefore:   32,
			expectedCapacityBefore: 32,
			expectedLengthAfter:    64,
			expectedCapacityAfter:  64,
		},
		{
			name:                   "data partial copy",
			memoryOffset:           32,
			dataOffset:             10,
			dataLength:             36,
			expectedLengthBefore:   32,
			expectedCapacityBefore: 32,
			expectedLengthAfter:    96,
			expectedCapacityAfter:  96,
		},
	}

	for _, test := range testCases {
		t.Setenv("name", test.name)

		s, closeFn := getState()
		defer closeFn()

		// set config
		cfg := chain.AllForksEnabled.At(0)
		s.config = &cfg

		// offset
		memoryOffset := test.memoryOffset
		dataOffset := test.dataOffset
		dataLength := test.dataLength

		// gas and code
		s.gas = 1000
		s.code = []byte{RETURNDATACOPY}

		s.returnData = make([]byte, (dataOffset + dataLength))
		for i := dataOffset; i < dataLength; i++ {
			s.returnData[i] = byte(i)
		}

		// initial state, which might set memory in other opcode
		v := s.extendMemory(big.NewInt(0), big.NewInt(memoryOffset))
		assert.True(t, v)
		assert.Equal(t, test.expectedLengthBefore, len(s.memory))
		assert.Equal(t, test.expectedCapacityBefore, cap(s.memory))

		// LIFO
		s.push(big.NewInt(dataLength))   // data length
		s.push(big.NewInt(dataOffset))   // data offset
		s.push(big.NewInt(memoryOffset)) // memory offset

		// run opcode
		_, err := s.Run()
		assert.NoError(t, err)
		assert.Len(t, s.returnData, int(test.dataOffset+test.dataLength))

		// final state
		assert.Equal(t, test.expectedLengthAfter, len(s.memory))
		assert.Equal(t, test.expectedCapacityAfter, cap(s.memory))
		assert.Equal(
			t,
			s.memory[memoryOffset:memoryOffset+dataLength],
			s.returnData[test.dataOffset:test.dataOffset+test.dataLength],
		)
	}
}
