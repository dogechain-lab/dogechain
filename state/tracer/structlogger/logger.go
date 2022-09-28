package structlogger

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"time"

	"github.com/dogechain-lab/dogechain/state/runtime"
	"github.com/dogechain-lab/dogechain/state/runtime/evm"
	"github.com/dogechain-lab/dogechain/types"
)

// Storage represents a contract's storage.
type Storage map[types.Hash]types.Hash

// Copy duplicates the current storage.
func (s Storage) Copy() Storage {
	cpy := make(Storage)
	for key, value := range s {
		cpy[key] = value
	}

	return cpy
}

// StructLog is emitted to the EVM each cycle and lists information about the current internal state
// prior to the execution of the statement.
type StructLog struct {
	Pc            uint64                    `json:"pc"`
	Op            int                       `json:"op"`
	Gas           uint64                    `json:"gas"`
	GasCost       uint64                    `json:"gasCost"`
	Memory        []byte                    `json:"memory"`
	MemorySize    int                       `json:"memSize"`
	Stack         []*big.Int                `json:"stack"`
	ReturnData    []byte                    `json:"returnData"`
	Storage       map[types.Hash]types.Hash `json:"-"`
	Depth         int                       `json:"depth"`
	RefundCounter uint64                    `json:"refund"`
	Err           error                     `json:"-"`
	OpName        string                    `json:"opName"`
	ErrorString   string                    `json:"error"`
}

func (s StructLog) MarshalJSON() ([]byte, error) {
	s.OpName = s.GetOpName()
	s.ErrorString = s.GetErrorString()

	return json.Marshal(&s)
}

// GetOpName formats the operand name in a human-readable format.
func (s *StructLog) GetOpName() string {
	return opInt2CodeName(s.Op)
}

// GetErrorString formats the log's error as a string.
func (s *StructLog) GetErrorString() string {
	if s.Err != nil {
		return s.Err.Error()
	}

	return ""
}

// StructLogger is an EVM state logger and implements EVMLogger.
//
// StructLogger can capture state based on the given Log configuration and also keeps
// a track record of modified storage which is used in reporting snapshots of the
// contract their storage.
type StructLogger struct {
	txn runtime.Txn

	storage map[types.Address]Storage
	logs    []*StructLog
	output  []byte
	err     error
}

// NewStructLogger returns a new logger
func NewStructLogger(txn runtime.Txn) *StructLogger {
	logger := &StructLogger{
		txn:     txn,
		storage: make(map[types.Address]Storage),
	}

	return logger
}

// Reset clears the data held by the logger.
func (l *StructLogger) Reset() {
	l.storage = make(map[types.Address]Storage)
	l.output = make([]byte, 0)
	l.logs = l.logs[:0]
	l.err = nil
}

// CaptureStart implements the EVMLogger interface to initialize the tracing operation.
func (l *StructLogger) CaptureStart(txn runtime.Txn, from, to types.Address,
	create bool, input []byte, gas uint64, value *big.Int) {
	l.txn = txn
}

// CaptureState logs a new structured log message and pushes it out to the environment
//
// CaptureState also tracks SLOAD/SSTORE ops to track storage change.
func (l *StructLogger) CaptureState(
	ctx *runtime.ScopeContext,
	pc uint64,
	opCode int,
	gas, cost uint64,
	rData []byte,
	depth int,
	err error,
) {
	// memory := ctx.Memory
	stack := ctx.Stack
	contractAddress := ctx.ContractAddress

	// // Copy a snapshot of the current memory state to a new buffer
	// mem := make([]byte, len(memory))
	// copy(mem, memory)

	// Copy a snapshot of the current stack state to a new buffer
	stck := make([]*big.Int, len(stack))
	for i, item := range stack {
		stck[i] = new(big.Int).SetBytes(item.Bytes())
	}

	// Copy stack data
	stackData := stack
	stackLen := len(stackData)

	// Copy a snapshot of the current storage to a new container
	var storage Storage
	if opCode == evm.SLOAD || opCode == evm.SSTORE {
		// initialise new changed values storage container for this contract
		// if not present.
		if l.storage[contractAddress] == nil {
			l.storage[contractAddress] = make(Storage)
		}

		var (
			address, value types.Hash
		)

		switch opCode {
		case evm.SLOAD:
			if stackLen < 1 {
				break
			}

			// capture SLOAD opcodes and record the read entry in the local storage
			address = types.BytesToHash(stackData[stackLen-1].Bytes())
			value = l.txn.GetState(contractAddress, address)
			l.storage[contractAddress][address] = value
			storage = l.storage[contractAddress].Copy()
		case evm.SSTORE:
			if stackLen < 2 {
				break
			}

			// capture SSTORE opcodes and record the written entry in the local storage.
			value = types.BytesToHash(stackData[stackLen-2].Bytes())
			address = types.BytesToHash(stackData[stackLen-1].Bytes())
			l.storage[contractAddress][address] = value
			storage = l.storage[contractAddress].Copy()
		}
	}

	// Copy return data
	rdata := make([]byte, len(rData))
	copy(rdata, rData)

	// create a new snapshot of the EVM.
	l.logs = append(l.logs, &StructLog{
		Pc:      pc,
		Op:      opCode,
		Gas:     gas,
		GasCost: cost,
		// Memory:        mem,
		// MemorySize:    len(memory),
		Stack:         stck,
		ReturnData:    rdata,
		Storage:       storage,
		Depth:         depth,
		RefundCounter: l.txn.GetRefund(),
		Err:           err,
	})
}

func (l *StructLogger) CaptureEnter(opCode int, from, to types.Address,
	input []byte, gas uint64, value *big.Int) {
}

func (l *StructLogger) CaptureExit(output []byte, gasUsed uint64, err error) {}

// CaptureFault implements the EVMLogger interface to trace an execution fault
// while running an opcode.
func (l *StructLogger) CaptureFault(ctx *runtime.ScopeContext, pc uint64, opCode int, gas, cost uint64,
	depth int, err error) {
}

// CaptureEnd is called after the call finishes to finalize the tracing.
func (l *StructLogger) CaptureEnd(output []byte, gasUsed uint64, t time.Duration, err error) {
	l.output = output
	l.err = err
}

// StructLogs returns the captured log entries.
func (l *StructLogger) StructLogs() []*StructLog { return l.logs }

// Error returns the VM error captured by the trace.
func (l *StructLogger) Error() error { return l.err }

// Output returns the VM return value captured by the trace.
func (l *StructLogger) Output() []byte { return l.output }

func opInt2CodeName(op int) string {
	return (evm.OpCode(op)).String()
}

// WriteTrace writes a formatted trace to the given writer
func WriteTrace(writer io.Writer, logs []StructLog) {
	for _, log := range logs {
		fmt.Fprintf(writer, "%-16spc=%08d gas=%v cost=%v",
			opInt2CodeName(log.Op), log.Pc, log.Gas, log.GasCost)

		if log.Err != nil {
			fmt.Fprintf(writer, " ERROR: %v", log.Err)
		}

		fmt.Fprintln(writer)

		if len(log.Stack) > 0 {
			fmt.Fprintln(writer, "Stack:")

			for i := len(log.Stack) - 1; i >= 0; i-- {
				fmt.Fprintf(writer, "%08d  %s\n", len(log.Stack)-i-1, log.Stack[i].Text(16))
			}
		}

		if len(log.Memory) > 0 {
			fmt.Fprintln(writer, "Memory:")
			fmt.Fprint(writer, hex.Dump(log.Memory))
		}

		if len(log.Storage) > 0 {
			fmt.Fprintln(writer, "Storage:")

			for h, item := range log.Storage {
				fmt.Fprintf(writer, "%x: %x\n", h, item)
			}
		}

		if len(log.ReturnData) > 0 {
			fmt.Fprintln(writer, "ReturnData:")
			fmt.Fprint(writer, hex.Dump(log.ReturnData))
		}

		fmt.Fprintln(writer)
	}
}

// WriteLogs writes vm logs in a readable format to the given writer
func WriteLogs(writer io.Writer, logs []*types.Log) {
	for _, log := range logs {
		// TODO: No block number in log and transaction index
		// fmt.Fprintf(writer, "LOG%d: %x bn=%d txi=%x\n",
		// len(log.Topics), log.Address, log., log)
		fmt.Fprintf(writer, "LOG%d: %x\n",
			len(log.Topics), log.Address)

		for i, topic := range log.Topics {
			fmt.Fprintf(writer, "%08d  %x\n", i, topic)
		}

		fmt.Fprint(writer, hex.Dump(log.Data))
		fmt.Fprintln(writer)
	}
}
