package tracer

import (
	"encoding/json"
	"errors"
	"math/big"
	"time"

	"github.com/dogechain-lab/dogechain/state/runtime"
	"github.com/dogechain-lab/dogechain/types"
)

// Context contains some contextual infos for a transaction execution that is not
// available from within the EVM object.
type Context struct {
	BlockHash types.Hash // Hash of the block the tx is contained within (zero if dangling tx or call)
	TxIndex   int        // Index of the transaction within a block (zero if dangling tx or call)
	TxHash    types.Hash // Hash of the transaction being traced (zero if dangling call)
}

// Tracer interface extends evm.EVMLogger and additionally
// allows collecting the tracing result.
type Tracer interface {
	runtime.EVMLogger
	GetResult() (json.RawMessage, error)
	// Stop terminates execution of the tracer at the first opportune moment.
	Stop(err error)
}

// dummyTracer does nothing in tracing
type dummyTracer struct{}

func NewDummyTracer() Tracer {
	return &dummyTracer{}
}

func (d *dummyTracer) CaptureStart(txn runtime.Txn, from, to types.Address, create bool,
	input []byte, gas uint64, value *big.Int) {
}
func (d *dummyTracer) CaptureState(ctx runtime.ScopeContext, pc uint64, op int,
	gas, cost uint64, rData []byte, depth int, err error) {
}
func (d *dummyTracer) CaptureEnter(typ int, from, to types.Address,
	input []byte, gas uint64, value *big.Int) {
}
func (d *dummyTracer) CaptureExit(output []byte, gasUsed uint64, err error) {
}
func (d *dummyTracer) CaptureFault(ctx runtime.ScopeContext, pc uint64, op int,
	gas, cost uint64, depth int, err error) {
}
func (d *dummyTracer) CaptureEnd(output []byte, gasUsed uint64, t time.Duration, err error) {
}
func (d *dummyTracer) GetResult() (json.RawMessage, error) {
	return nil, nil
}
func (d *dummyTracer) Stop(err error) {}

type lookupFunc func(string, *Context) (Tracer, error)

var (
	lookups []lookupFunc
)

// RegisterLookup registers a method as a lookup for tracers, meaning that
// users can invoke a named tracer through that lookup. If 'wildcard' is true,
// then the lookup will be placed last. This is typically meant for interpreted
// engines (js) which can evaluate dynamic user-supplied code.
func RegisterLookup(wildcard bool, lookup lookupFunc) {
	if wildcard {
		lookups = append(lookups, lookup)
	} else {
		lookups = append([]lookupFunc{lookup}, lookups...)
	}
}

// New returns a new instance of a tracer, by iterating through the
// registered lookups.
func New(code string, ctx *Context) (Tracer, error) {
	for _, lookup := range lookups {
		if tracer, err := lookup(code, ctx); err == nil {
			return tracer, nil
		}
	}

	return nil, errors.New("tracer not found")
}
