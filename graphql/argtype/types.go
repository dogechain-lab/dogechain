package argtype

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/dogechain-lab/dogechain/helper/hex"
	rpc "github.com/dogechain-lab/dogechain/jsonrpc"
	"github.com/dogechain-lab/dogechain/types"
)

type Long int64

// ImplementsGraphQLType returns true if Long implements the provided GraphQL type.
func (b Long) ImplementsGraphQLType(name string) bool { return name == "Long" }

// UnmarshalGraphQL unmarshals the provided GraphQL query data.
func (b *Long) UnmarshalGraphQL(input interface{}) error {
	var err error
	switch input := input.(type) {
	case string:
		// uncomment to support hex values
		value, err := strconv.ParseInt(input, 10, 64)
		*b = Long(value)

		return err
		//}
	case int32:
		*b = Long(input)
	case int64:
		*b = Long(input)
	case float64:
		*b = Long(input)
	default:
		err = fmt.Errorf("unexpected type %T for Long", input)
	}

	return err
}

type Big big.Int

func ArgBigPtr(b *big.Int) *Big {
	v := Big(*b)

	return &v
}

func (b *Big) UnmarshalText(input []byte) error {
	buf, err := decodeToHex(input)
	if err != nil {
		return err
	}

	c := new(big.Int)
	c.SetBytes(buf)
	*b = Big(*c)

	return nil
}

func (b Big) MarshalText() ([]byte, error) {
	c := (*big.Int)(&b)

	return []byte("0x" + c.Text(16)), nil
}

// ImplementsGraphQLType returns true if Big implements the provided GraphQL type.
func (b Big) ImplementsGraphQLType(name string) bool { return name == "BigInt" }

// UnmarshalGraphQL unmarshals the provided GraphQL query data.
func (b *Big) UnmarshalGraphQL(input interface{}) error {
	var err error

	switch input := input.(type) {
	case string:
		return b.UnmarshalText([]byte(input))
	case int32:
		var num big.Int

		num.SetInt64(int64(input))
		*b = Big(num)
	default:
		err = fmt.Errorf("unexpected type %T for BigInt", input)
	}

	return err
}

func AddrPtr(a types.Address) *types.Address {
	return &a
}

func HashPtr(h types.Hash) *types.Hash {
	return &h
}

type Uint64 uint64

func UintPtr(n uint64) *Uint64 {
	v := Uint64(n)

	return &v
}

func (u Uint64) MarshalText() ([]byte, error) {
	buf := make([]byte, 2, 10)
	copy(buf, `0x`)
	buf = strconv.AppendUint(buf, uint64(u), 16)

	return buf, nil
}

func (u *Uint64) UnmarshalText(input []byte) error {
	str := strings.TrimPrefix(string(input), "0x")
	num, err := strconv.ParseUint(str, 16, 64)

	if err != nil {
		return err
	}

	*u = Uint64(num)

	return nil
}

// ImplementsGraphQLType returns true if Uint64 implements the provided GraphQL type.
func (u Uint64) ImplementsGraphQLType(name string) bool { return name == "Long" }

// UnmarshalGraphQL unmarshals the provided GraphQL query data.
func (u *Uint64) UnmarshalGraphQL(input interface{}) error {
	var err error

	switch input := input.(type) {
	case string:
		return u.UnmarshalText([]byte(input))
	case int32:
		*u = Uint64(input)
	default:
		err = fmt.Errorf("unexpected type %T for Long", input)
	}

	return err
}

type Bytes []byte

func BytesPtr(b []byte) *Bytes {
	bb := Bytes(b)

	return &bb
}

func (b Bytes) MarshalText() ([]byte, error) {
	return encodeToHex(b), nil
}

func (b *Bytes) UnmarshalText(input []byte) error {
	hh, err := decodeToHex(input)
	if err != nil {
		return nil
	}

	aux := make([]byte, len(hh))
	copy(aux[:], hh[:])
	*b = aux

	return nil
}

// ImplementsGraphQLType returns true if Bytes implements the specified GraphQL type.
func (b Bytes) ImplementsGraphQLType(name string) bool { return name == "Bytes" }

// UnmarshalGraphQL unmarshals the provided GraphQL query data.
func (b *Bytes) UnmarshalGraphQL(input interface{}) error {
	var err error

	switch input := input.(type) {
	case string:
		data, err := hex.DecodeString(input)
		if err != nil {
			return err
		}

		*b = data
	default:
		err = fmt.Errorf("unexpected type %T for Bytes", input)
	}

	return err
}

func decodeToHex(b []byte) ([]byte, error) {
	str := string(b)
	str = strings.TrimPrefix(str, "0x")

	if len(str)%2 != 0 {
		str = "0" + str
	}

	return hex.DecodeString(str)
}

func encodeToHex(b []byte) []byte {
	str := hex.EncodeToString(b)
	if len(str)%2 != 0 {
		str = "0" + str
	}

	return []byte("0x" + str)
}

type Log struct {
	Address     types.Address `json:"address"`
	Topics      []types.Hash  `json:"topics"`
	Data        Bytes         `json:"data"`
	BlockNumber Uint64        `json:"blockNumber"`
	TxHash      types.Hash    `json:"transactionHash"`
	TxIndex     Uint64        `json:"transactionIndex"`
	BlockHash   types.Hash    `json:"blockHash"`
	LogIndex    Uint64        `json:"logIndex"`
	Removed     bool          `json:"removed"`
}

func FromRPCLog(log *rpc.Log) *Log {
	return &Log{
		Address:     log.Address,
		Topics:      log.Topics,
		Data:        Bytes(log.Data),
		BlockNumber: Uint64(log.BlockNumber),
		TxHash:      log.TxHash,
		TxIndex:     Uint64(log.TxIndex),
		BlockHash:   log.BlockHash,
		LogIndex:    Uint64(log.LogIndex),
		Removed:     log.Removed,
	}
}
