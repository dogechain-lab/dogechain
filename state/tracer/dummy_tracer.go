package tracer

import (
	"encoding/json"
	"math/big"
	"time"

	"github.com/dogechain-lab/dogechain/state/runtime"
	"github.com/dogechain-lab/dogechain/types"
)

// dummyTracer does nothing in tracing
type dummyTracer struct{}

func NewDummyTracer() Tracer {
	return &dummyTracer{}
}

func (d *dummyTracer) CaptureStart(txn runtime.Txn, from, to types.Address, create bool,
	input []byte, gas uint64, value *big.Int) {
}
func (d *dummyTracer) CaptureState(ctx runtime.ScopeContext, pc uint64, opCode int,
	gas, cost uint64, rData []byte, depth int, err error) {
}
func (d *dummyTracer) CaptureEnter(opCode int, from, to types.Address,
	input []byte, gas uint64, value *big.Int) {
}
func (d *dummyTracer) CaptureExit(output []byte, gasUsed uint64, err error) {
}
func (d *dummyTracer) CaptureFault(ctx runtime.ScopeContext, pc uint64, opCode int,
	gas, cost uint64, depth int, err error) {
}
func (d *dummyTracer) CaptureEnd(output []byte, gasUsed uint64, t time.Duration, err error) {
}
func (d *dummyTracer) GetResult() (json.RawMessage, error) {
	return nil, nil
}
func (d *dummyTracer) Stop(err error) {}
