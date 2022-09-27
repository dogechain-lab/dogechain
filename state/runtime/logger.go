package runtime

import (
	"math/big"
	"time"

	"github.com/dogechain-lab/dogechain/types"
)

// Txn is used to get an txn in current transition
type Txn interface {
	GetState(addr types.Address, key types.Hash) types.Hash
	GetRefund() uint64
}

// ScopeContext contains the things that are per-call, such as stack and memory,
// but not transients like pc and gas
type ScopeContext struct {
	Memory          []byte
	Stack           []*big.Int
	ContractAddress types.Address
}

// EVMLogger is used to collect execution traces from an EVM transaction execution.
// CaptureState is called for each step of the VM with the current VM state.
// Note that reference types are actual VM data structures; make copies if you need to
// retain them beyond the current call.
type EVMLogger interface {
	CaptureStart(txn Txn, from, to types.Address, create bool, input []byte, gas uint64, value *big.Int)
	CaptureState(ctx *ScopeContext, pc uint64, opCode int, gas, cost uint64, rData []byte, depth int, err error)
	CaptureEnter(opCode int, from, to types.Address, input []byte, gas uint64, value *big.Int)
	CaptureExit(output []byte, gasUsed uint64, err error)
	CaptureFault(ctx *ScopeContext, pc uint64, opCode int, gas, cost uint64, depth int, err error)
	CaptureEnd(output []byte, gasUsed uint64, t time.Duration, err error)
}
