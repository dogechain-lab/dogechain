package jsonrpc

import (
	"math/big"
	"testing"

	"github.com/dogechain-lab/dogechain/helper/hex"
	"github.com/dogechain-lab/dogechain/state/runtime/evm"
	"github.com/dogechain-lab/dogechain/state/tracer/structlogger"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/stretchr/testify/assert"
)

func TestDebug_FormatLogs(t *testing.T) {
	var (
		stackPc121 = []string{
			"0x1ab06ee5",
			"0x55",
			"0x4",
			"0x5",
			"0x4",
			"0x0",
		}
		stackPc122 = []string{
			"0x1ab06ee5",
			"0x55",
			"0x4",
			"0x5",
			"0x4",
			"0x0",
			"0x4",
		}
		stackPc123 = []string{
			"0x1ab06ee5",
			"0x55",
			"0x4",
			"0x5",
			"0x4",
			"0x4",
			"0x0",
		}
		stackPc124 = []string{
			"0x1ab06ee5",
			"0x55",
			"0x4",
			"0x5",
			"0x4",
		}
		storagePc123 = map[string]string{
			"0000000000000000000000000000000000000000000000000000000000000000": "0000000000000000000000000000000000000000000000000000000000000004",
		}
		memoryBytesPc124, _ = hex.DecodeHex("0000000000000000000000000000000000000000000000000000000000000000" +
			"0000000000000000000000000000000000000000000000000000000000000000" +
			"0000000000000000000000000000000000000000000000000000000000000080")
		memoryPc124 = []string{
			"0000000000000000000000000000000000000000000000000000000000000000",
			"0000000000000000000000000000000000000000000000000000000000000000",
			"0000000000000000000000000000000000000000000000000000000000000080",
		}
	)

	tests := []struct {
		name   string
		input  []*structlogger.StructLog
		result []StructLogRes
	}{
		{
			name: "Stack should format right",
			input: []*structlogger.StructLog{
				{
					Pc:      121,
					Op:      evm.DUP1 + 1, // DUP2
					Gas:     40035,
					GasCost: 3,
					Depth:   1,
					Stack: []*big.Int{
						new(big.Int).SetUint64(0x1ab06ee5),
						new(big.Int).SetUint64(0x55),
						new(big.Int).SetUint64(0x4),
						new(big.Int).SetUint64(0x5),
						new(big.Int).SetUint64(0x4),
						new(big.Int).SetUint64(0x0),
					},
				},
				{
					Pc:      122,
					Op:      evm.SWAP1,
					Gas:     40032,
					GasCost: 3,
					Depth:   1,
					Stack: []*big.Int{
						new(big.Int).SetUint64(0x1ab06ee5),
						new(big.Int).SetUint64(0x55),
						new(big.Int).SetUint64(0x4),
						new(big.Int).SetUint64(0x5),
						new(big.Int).SetUint64(0x4),
						new(big.Int).SetUint64(0x0),
						new(big.Int).SetUint64(0x4),
					},
				},
			},
			result: []StructLogRes{
				{
					Pc:      121,
					Op:      evm.OpCode(evm.DUP1 + 1).String(), // DUP2
					Gas:     40035,
					GasCost: 3,
					Depth:   1,
					Stack:   &stackPc121,
				},
				{
					Pc:      122,
					Op:      evm.OpCode(evm.SWAP1).String(),
					Gas:     40032,
					GasCost: 3,
					Depth:   1,
					Stack:   &stackPc122,
				},
			},
		},
		{
			name: "Storage should format right",
			input: []*structlogger.StructLog{
				{
					Pc:      123,
					Op:      evm.SSTORE,
					Gas:     40029,
					GasCost: 20000,
					Depth:   1,
					Stack: []*big.Int{
						new(big.Int).SetUint64(0x1ab06ee5),
						new(big.Int).SetUint64(0x55),
						new(big.Int).SetUint64(0x4),
						new(big.Int).SetUint64(0x5),
						new(big.Int).SetUint64(0x4),
						new(big.Int).SetUint64(0x4),
						new(big.Int).SetUint64(0x0),
					},
					Storage: map[types.Hash]types.Hash{
						types.StringToHash("0x0"): types.StringToHash("0x4"),
					},
				},
			},
			result: []StructLogRes{
				{
					Pc:      123,
					Op:      evm.OpCode(evm.SSTORE).String(),
					Gas:     40029,
					GasCost: 20000,
					Depth:   1,
					Stack:   &stackPc123,
					Storage: &storagePc123,
				},
			},
		},
		{
			name: "Memory should format right",
			input: []*structlogger.StructLog{
				{
					Pc:      124,
					Op:      evm.POP,
					Gas:     20029,
					GasCost: 2,
					Depth:   1,
					Stack: []*big.Int{
						new(big.Int).SetUint64(0x1ab06ee5),
						new(big.Int).SetUint64(0x55),
						new(big.Int).SetUint64(0x4),
						new(big.Int).SetUint64(0x5),
						new(big.Int).SetUint64(0x4),
					},
					Memory: memoryBytesPc124,
				},
			},
			result: []StructLogRes{
				{
					Pc:      124,
					Op:      evm.OpCode(evm.POP).String(),
					Gas:     20029,
					GasCost: 2,
					Depth:   1,
					Stack:   &stackPc124,
					Memory:  &memoryPc124,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// format it
			v := formatLogs(tt.input)

			// Assert equality
			assert.Equal(t, tt.result, v)
		})
	}
}
