package jsonrpc

import (
	"errors"
	"fmt"

	"github.com/dogechain-lab/dogechain/helper/hex"
	"github.com/dogechain-lab/dogechain/state"
	"github.com/dogechain-lab/dogechain/state/runtime"
	"github.com/dogechain-lab/dogechain/state/tracer/structlogger"
	"github.com/dogechain-lab/dogechain/types"
)

var (
	ErrTransactionNotSeal         = errors.New("transaction not sealed")
	ErrGenesisNotTracable         = errors.New("genesis is not traceable")
	ErrTransactionNotFoundInBlock = errors.New("transaction not found in block")
)

type Debug struct {
	store ethStore
}

func (d *Debug) TraceTransaction(hash types.Hash) (interface{}, error) {
	// Check the chain state for the transaction
	blockHash, ok := d.store.ReadTxLookup(hash)
	if !ok {
		// Block not found in storage
		return nil, ErrBlockNotFound
	}

	block, ok := d.store.GetBlockByHash(blockHash, true)
	if !ok {
		// Block receipts not found in storage
		return nil, ErrTransactionNotSeal
	}
	// It shouldn't happen in practice.
	if block.Number() == 0 {
		return nil, ErrGenesisNotTracable
	}

	var (
		tx    *types.Transaction
		txIdx = -1
	)

	// Find the transaction within the block
	for idx, txn := range block.Transactions {
		if txn.Hash() == hash {
			tx = txn
			txIdx = idx

			break
		}
	}

	if txIdx < 0 {
		// it shouldn't be
		return nil, ErrTransactionNotFoundInBlock
	}

	txn, err := d.store.StateAtTransaction(block, txIdx)
	if err != nil {
		return nil, err
	}

	return d.traceTx(txn, tx)
}

func (d *Debug) traceTx(txn *state.Transition, tx *types.Transaction) (interface{}, error) {
	var tracer runtime.EVMLogger = structlogger.NewStructLogger(txn.Txn())

	txn.SetEVMLogger(tracer)

	result, err := txn.Apply(tx)
	if err != nil {
		return nil, fmt.Errorf("tracing failed: %w", err)
	}

	switch tracer := tracer.(type) {
	case *structlogger.StructLogger:
		returnVal := fmt.Sprintf("%x", result.Return())
		// If the result contains a revert reason, return it.
		if result.Reverted() {
			returnVal = fmt.Sprintf("%x", result.Revert())
		}

		return &ExecutionResult{
			Gas:         result.GasUsed,
			Failed:      result.Failed(),
			ReturnValue: returnVal,
			StructLogs:  formatLogs(tracer.StructLogs()),
		}, nil
	default:
		panic(fmt.Sprintf("bad tracer type %T", tracer))
	}
}

// ExecutionResult groups all structured logs emitted by the EVM
// while replaying a transaction in debug mode as well as transaction
// execution status, the amount of gas used and the return value
type ExecutionResult struct {
	Gas         uint64         `json:"gas"`
	Failed      bool           `json:"failed"`
	ReturnValue string         `json:"returnValue"`
	StructLogs  []StructLogRes `json:"structLogs"`
}

// StructLogRes stores a structured log emitted by the EVM while replaying a
// transaction in debug mode
type StructLogRes struct {
	Pc      uint64             `json:"pc"`
	Op      string             `json:"op"`
	Gas     uint64             `json:"gas"`
	GasCost uint64             `json:"gasCost"`
	Depth   int                `json:"depth"`
	Error   string             `json:"error,omitempty"`
	Stack   *[]string          `json:"stack,omitempty"`
	Memory  *[]string          `json:"memory,omitempty"`
	Storage *map[string]string `json:"storage,omitempty"`
}

// formatLogs formats EVM returned structured logs for json output
func formatLogs(logs []*structlogger.StructLog) []StructLogRes {
	formatted := make([]StructLogRes, len(logs))

	for index, trace := range logs {
		trace.OpName = trace.GetOpName()
		trace.ErrorString = trace.GetErrorString()

		formatted[index] = StructLogRes{
			Pc:      trace.Pc,
			Op:      trace.OpName,
			Gas:     trace.Gas,
			GasCost: trace.GasCost,
			Depth:   trace.Depth,
			Error:   trace.ErrorString,
		}

		if trace.Stack != nil {
			stack := make([]string, len(trace.Stack))
			for i, stackValue := range trace.Stack {
				stack[i] = hex.EncodeBig(stackValue)
			}

			formatted[index].Stack = &stack
		}

		if len(trace.Memory) > 0 {
			memory := make([]string, 0, (len(trace.Memory)+31)/32)
			for i := 0; i+32 <= len(trace.Memory); i += 32 {
				memory = append(memory, hex.EncodeToString(trace.Memory[i:i+32]))
			}

			formatted[index].Memory = &memory
		}

		if len(trace.Storage) > 0 {
			storage := make(map[string]string)
			for addr, val := range trace.Storage {
				storage[hex.EncodeToString(addr.Bytes())] = hex.EncodeToString(val.Bytes())
			}

			formatted[index].Storage = &storage
		}
	}

	return formatted
}
