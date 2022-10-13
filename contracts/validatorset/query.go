package validatorset

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/dogechain-lab/dogechain/contracts/abis"
	"github.com/dogechain-lab/dogechain/contracts/systemcontracts"
	"github.com/dogechain-lab/dogechain/state/runtime"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/umbracle/go-web3"
	"github.com/umbracle/go-web3/abi"
)

const (
	// method
	_validatorsMethodName = "validators"
	_depositMethodName    = "deposit"
	_slashMethodName      = "slash"
)

const (
	// parameter name
	_depositParameterName = "validatorAddress"
	_slashParameterName   = "array"
)

const (
	// Gas limit used when querying the validator set
	_queryGasLimit uint64 = 2000000
)

func DecodeValidators(method *abi.Method, returnValue []byte) ([]types.Address, error) {
	decodedResults, err := method.Outputs.Decode(returnValue)
	if err != nil {
		return nil, err
	}

	results, ok := decodedResults.(map[string]interface{})
	if !ok {
		return nil, errors.New("failed type assertion from decodedResults to map")
	}

	web3Addresses, ok := results["0"].([]web3.Address)

	if !ok {
		return nil, errors.New("failed type assertion from results[0] to []web3.Address")
	}

	addresses := make([]types.Address, len(web3Addresses))
	for idx, waddr := range web3Addresses {
		addresses[idx] = types.Address(waddr)
	}

	return addresses, nil
}

type TxQueryHandler interface {
	Apply(*types.Transaction) (*runtime.ExecutionResult, error)
	GetNonce(types.Address) uint64
}

func QueryValidators(t TxQueryHandler, from types.Address) ([]types.Address, error) {
	method, ok := abis.ValidatorSetABI.Methods[_validatorsMethodName]
	if !ok {
		return nil, fmt.Errorf("validatorset contract ABI no %s method", _validatorsMethodName)
	}

	selector := method.ID()
	res, err := t.Apply(&types.Transaction{
		From:     from,
		To:       &systemcontracts.AddrValidatorSetContract,
		Value:    big.NewInt(0),
		Input:    selector,
		GasPrice: big.NewInt(0),
		Gas:      _queryGasLimit,
		Nonce:    t.GetNonce(from),
	})

	if err != nil {
		return nil, err
	}

	if res.Failed() {
		return nil, res.Err
	}

	return DecodeValidators(method, res.ReturnValue)
}

func MakeDepositTx(t TxQueryHandler, from types.Address) (*types.Transaction, error) {
	method, ok := abis.ValidatorSetABI.Methods[_depositMethodName]
	if !ok {
		return nil, fmt.Errorf("validatorset contract ABI no %s method", _depositMethodName)
	}

	inputs, err := method.Inputs.Encode(
		map[string]interface{}{
			_depositParameterName: from,
		},
	)
	if err != nil {
		return nil, err
	}

	tx := &types.Transaction{
		Nonce:    t.GetNonce(from),
		GasPrice: big.NewInt(0),
		Gas:      _queryGasLimit,
		To:       &systemcontracts.AddrValidatorSetContract,
		Value:    nil,
		Input:    append(method.ID(), inputs...),
		From:     from,
	}

	return tx, nil
}

func MakeSlashTx(t TxQueryHandler, from types.Address, needPunished []types.Address) (*types.Transaction, error) {
	method, ok := abis.ValidatorSetABI.Methods[_slashMethodName]
	if !ok {
		return nil, fmt.Errorf("validatorset contract ABI no %s method", _slashMethodName)
	}

	encodedInput, err := method.Inputs.Encode(
		map[string]interface{}{
			_slashParameterName: needPunished,
		},
	)
	if err != nil {
		return nil, err
	}

	tx := &types.Transaction{
		Nonce:    t.GetNonce(from),
		GasPrice: big.NewInt(0),
		Gas:      _queryGasLimit,
		To:       &systemcontracts.AddrValidatorSetContract,
		Value:    nil,
		Input:    append(method.ID(), encodedInput...),
		From:     from,
	}

	return tx, nil
}
