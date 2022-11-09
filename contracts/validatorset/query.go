package validatorset

import (
	"bytes"
	"errors"
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
	_slashParameterName   = "validatorAddress"
)

const (
	// Gas limit used when querying the validator set
	SystemTransactionGasLimit uint64 = 100_000
)

var (
	// some important reuse variable. must exists
	_depositMethodID = abis.ValidatorSetABI.Methods[_depositMethodName].ID()
	_slashMethodID   = abis.ValidatorSetABI.Methods[_slashMethodName].ID()
)

func DecodeValidators(method *abi.Method, returnValue []byte) ([]types.Address, error) {
	results, err := abis.DecodeTxMethodOutput(method, returnValue)
	if err != nil {
		return nil, err
	}

	// type assertion
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

type NonceHub interface {
	GetNonce(types.Address) uint64
}

type TxQueryHandler interface {
	NonceHub
	Apply(*types.Transaction) (*runtime.ExecutionResult, error)
}

func QueryValidators(t TxQueryHandler, from types.Address, gasLimit uint64) ([]types.Address, error) {
	method := abis.ValidatorSetABI.Methods[_validatorsMethodName]

	input, err := abis.EncodeTxMethod(method, nil)
	if err != nil {
		return nil, err
	}

	res, err := t.Apply(&types.Transaction{
		From:     from,
		To:       &systemcontracts.AddrValidatorSetContract,
		Value:    big.NewInt(0),
		Input:    input,
		GasPrice: big.NewInt(0),
		Gas:      gasLimit,
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

func MakeDepositTx(t NonceHub, from types.Address) (*types.Transaction, error) {
	method := abis.ValidatorSetABI.Methods[_depositMethodName]

	input, err := abis.EncodeTxMethod(method, nil)
	if err != nil {
		return nil, err
	}

	tx := &types.Transaction{
		Nonce:    t.GetNonce(from),
		GasPrice: big.NewInt(0),
		Gas:      SystemTransactionGasLimit,
		To:       &systemcontracts.AddrValidatorSetContract,
		Value:    nil,
		Input:    input,
		From:     from,
	}

	return tx, nil
}

func MakeSlashTx(t NonceHub, from types.Address, needPunished types.Address) (*types.Transaction, error) {
	method := abis.ValidatorSetABI.Methods[_slashMethodName]

	input, err := abis.EncodeTxMethod(
		method,
		map[string]interface{}{
			_slashParameterName: web3.Address(needPunished),
		},
	)
	if err != nil {
		return nil, err
	}

	tx := &types.Transaction{
		Nonce:    t.GetNonce(from),
		GasPrice: big.NewInt(0),
		Gas:      SystemTransactionGasLimit,
		To:       &systemcontracts.AddrValidatorSetContract,
		Value:    nil,
		Input:    input,
		From:     from,
	}

	return tx, nil
}

func IsDepositTransactionSignture(in []byte) bool {
	if len(in) < 4 {
		return false
	}

	return bytes.EqualFold(in[:4], _depositMethodID)
}

func IsSlashTransactionSignture(in []byte) bool {
	if len(in) != 36 { // methodid + address
		return false
	}

	return bytes.EqualFold(in[:4], _slashMethodID)
}
