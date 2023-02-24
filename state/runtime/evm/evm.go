package evm

import (
	"errors"
	"fmt"

	"github.com/dogechain-lab/dogechain/chain"
	"github.com/dogechain-lab/dogechain/state/runtime"
)

// runtime interface compatible
var _ runtime.Runtime = &EVM{}

// EVM is the ethereum virtual machine
type EVM struct {
}

// NewEVM creates a new EVM
func NewEVM() *EVM {
	return &EVM{}
}

// CanRun implements the runtime interface
func (e *EVM) CanRun(*runtime.Contract, runtime.Host, *chain.ForksInTime) bool {
	return true
}

// Name implements the runtime interface
func (e *EVM) Name() string {
	return "evm"
}

// Run implements the runtime interface
func (e *EVM) Run(c *runtime.Contract, host runtime.Host, config *chain.ForksInTime) *runtime.ExecutionResult {
	contract := acquireState()
	contract.resetReturnData()

	contract.msg = c
	contract.code = c.Code
	contract.evm = e
	contract.gas = c.Gas
	contract.host = host
	contract.config = config

	contract.bitmap.setCode(c.Code)

	gasBefore := c.Gas

	ret, err := contract.Run()

	fmt.Printf("debug: run contract(%s), depth(%d), value(%s), gas(%d), gasLeft(%d)\n", c.Address, c.Depth, c.Value, gasBefore, contract.gas)

	// We are probably doing this append magic to make sure that the slice doesn't have more capacity than it needs
	var returnValue []byte
	returnValue = append(returnValue[:0], ret...)

	gasLeft := contract.gas

	releaseState(contract)

	if err != nil && !errors.Is(err, errRevert) {
		gasLeft = 0
	}

	return &runtime.ExecutionResult{
		ReturnValue: returnValue,
		GasLeft:     gasLeft,
		Err:         err,
	}
}
