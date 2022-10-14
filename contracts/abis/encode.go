package abis

import (
	"bytes"
	"errors"

	"github.com/umbracle/go-web3/abi"
)

var (
	ErrNoMethod          = errors.New("no method in abi")
	ErrInvalidSignature  = errors.New("invalid signature")
	ErrResultTypeCasting = errors.New("failed to casting type to map")
)

func EncodeTxMethod(method *abi.Method, abiArgs map[string]interface{}) (input []byte, err error) {
	if method == nil {
		return nil, ErrNoMethod
	}

	input = append(input, method.ID()...)

	// no input
	if abiArgs == nil {
		return
	}

	// rlp marshal args
	args, err := method.Inputs.Encode(abiArgs)
	if err != nil {
		return nil, err
	}

	return append(input, args...), nil
}

func decodeABITypeVal(typ *abi.Type, val []byte) (map[string]interface{}, error) {
	result, err := typ.Decode(val)
	if err != nil {
		return nil, err
	}

	output, ok := result.(map[string]interface{})
	if !ok {
		return nil, ErrResultTypeCasting
	}

	return output, nil
}

func DecodeTxMethodInput(method *abi.Method, val []byte) (map[string]interface{}, error) {
	if method == nil {
		return nil, ErrNoMethod
	}

	if len(val) < 4 {
		return nil, ErrInvalidSignature
	}

	if !bytes.EqualFold(method.ID(), val[:4]) {
		return nil, ErrInvalidSignature
	}

	return decodeABITypeVal(method.Inputs, val[4:])
}

func DecodeTxMethodOutput(method *abi.Method, val []byte) (map[string]interface{}, error) {
	if method == nil {
		return nil, ErrNoMethod
	}

	return decodeABITypeVal(method.Outputs, val)
}
