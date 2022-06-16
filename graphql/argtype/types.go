package argtype

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/dogechain-lab/dogechain/helper/hex"
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

func (a *Big) UnmarshalText(input []byte) error {
	buf, err := DecodeToHex(input)
	if err != nil {
		return err
	}

	b := new(big.Int)
	b.SetBytes(buf)
	*a = Big(*b)

	return nil
}

func (a Big) MarshalText() ([]byte, error) {
	b := (*big.Int)(&a)

	return []byte("0x" + b.Text(16)), nil
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

type Bytes []byte

func BytesPtr(b []byte) *Bytes {
	bb := Bytes(b)

	return &bb
}

func (b Bytes) MarshalText() ([]byte, error) {
	return EncodeToHex(b), nil
}

func (b *Bytes) UnmarshalText(input []byte) error {
	hh, err := DecodeToHex(input)
	if err != nil {
		return nil
	}

	aux := make([]byte, len(hh))
	copy(aux[:], hh[:])
	*b = aux

	return nil
}

func DecodeToHex(b []byte) ([]byte, error) {
	str := string(b)
	str = strings.TrimPrefix(str, "0x")

	if len(str)%2 != 0 {
		str = "0" + str
	}

	return hex.DecodeString(str)
}

func EncodeToHex(b []byte) []byte {
	str := hex.EncodeToString(b)
	if len(str)%2 != 0 {
		str = "0" + str
	}

	return []byte("0x" + str)
}
