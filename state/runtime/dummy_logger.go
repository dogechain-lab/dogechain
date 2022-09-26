package runtime

import (
	"math/big"
	"time"

	"github.com/dogechain-lab/dogechain/types"
)

// dummyLogger does nothing in state logging
type dummyLogger struct{}

func NewDummyLogger() EVMLogger {
	return &dummyLogger{}
}

func (d *dummyLogger) CaptureStart(txn Txn, from, to types.Address, create bool,
	input []byte, gas uint64, value *big.Int) {
}
func (d *dummyLogger) CaptureState(ctx ScopeContext, pc uint64, opCode int,
	gas, cost uint64, rData []byte, depth int, err error) {
}
func (d *dummyLogger) CaptureEnter(opCode int, from, to types.Address,
	input []byte, gas uint64, value *big.Int) {
}
func (d *dummyLogger) CaptureExit(output []byte, gasUsed uint64, err error) {
}
func (d *dummyLogger) CaptureFault(ctx ScopeContext, pc uint64, opCode int,
	gas, cost uint64, depth int, err error) {
}
func (d *dummyLogger) CaptureEnd(output []byte, gasUsed uint64, t time.Duration, err error) {
}
