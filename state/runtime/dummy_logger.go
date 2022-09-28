package runtime

import (
	"math/big"
	"time"

	"github.com/dogechain-lab/dogechain/types"
)

// DummyLogger does nothing in state logging
type DummyLogger struct{}

func NewDummyLogger() EVMLogger {
	return &DummyLogger{}
}

func (d *DummyLogger) CaptureStart(txn Txn, from, to types.Address, create bool,
	input []byte, gas uint64, value *big.Int) {
}
func (d *DummyLogger) CaptureState(ctx *ScopeContext, pc uint64, opCode int,
	gas, cost uint64, rData []byte, depth int, err error) {
}
func (d *DummyLogger) CaptureEnter(opCode int, from, to types.Address,
	input []byte, gas uint64, value *big.Int) {
}
func (d *DummyLogger) CaptureExit(output []byte, gasUsed uint64, err error) {
}
func (d *DummyLogger) CaptureFault(ctx *ScopeContext, pc uint64, opCode int,
	gas, cost uint64, depth int, err error) {
}
func (d *DummyLogger) CaptureEnd(output []byte, gasUsed uint64, t time.Duration, err error) {
}
