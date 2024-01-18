package types

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"sync"
	"unicode"

	"github.com/dogechain-lab/dogechain/helper/hex"
	"github.com/dogechain-lab/dogechain/helper/keccak"
)

var ZeroAddress = Address{}

const (
	AddressLength = 20
)

type Address [AddressLength]byte

var runePool = sync.Pool{
	New: func() interface{} {
		return make([]rune, 0, AddressLength*2)
	},
}

// checksumEncode returns the checksummed address with 0x prefix, as by EIP-55
// https://github.com/ethereum/EIPs/blob/master/EIPS/eip-55.md
func (a Address) checksumEncode() string {
	addrBytes := a.Bytes() // 20 bytes

	// Encode to hex
	lowercaseHex := hex.EncodeToString(addrBytes)
	hashedAddress := hex.EncodeToString(keccak.Keccak256(nil, []byte(lowercaseHex)))

	var result []rune = nil

	{
		resultPtr, ok := runePool.Get().(*[]rune)
		if !ok {
			result := make([]rune, 0, AddressLength*2)
			resultPtr = &result
		}

		result = *resultPtr
	}

	defer func() {
		// cleanup the result slice
		result = result[0:0]
		runePool.Put(&result)
	}()

	// resize the result slice
	result = result[:len(lowercaseHex)]

	// Iterate over each character in the lowercase hex address
	for idx, ch := range lowercaseHex {
		if ch >= '0' && ch <= '9' || hashedAddress[idx] >= '0' && hashedAddress[idx] <= '7' {
			// Numbers in range [0, 9] are ignored (as well as hashed values [0, 7]),
			// because they can't be uppercased
			result[idx] = ch
		} else {
			// The current character / hashed character is in the range [8, f]
			result[idx] = unicode.ToUpper(ch)
		}
	}

	builder := new(strings.Builder)
	builder.WriteString(hex.HexPrefix)
	builder.WriteString(string(result))

	return builder.String()
}

func (a Address) Ptr() *Address {
	return &a
}

func (a Address) String() string {
	return a.checksumEncode()
}

func (a Address) Bytes() []byte {
	return a[:]
}

func (a Address) Value() (driver.Value, error) {
	return a.String(), nil
}

func (a *Address) Scan(src interface{}) error {
	stringVal, ok := src.([]byte)
	if !ok {
		return errors.New("invalid type assert")
	}

	aa, err := hex.DecodeHex(string(stringVal))
	if err != nil {
		return fmt.Errorf("decode hex err: %w", err)
	}

	copy(a[:], aa[:])

	return nil
}

func StringToAddress(str string) Address {
	return BytesToAddress(StringToBytes(str))
}

func AddressToString(address Address) string {
	return string(address[:])
}

func BytesToAddress(b []byte) Address {
	var a Address

	size := len(b)
	min := min(size, AddressLength)

	copy(a[AddressLength-min:], b[len(b)-min:])

	return a
}

// UnmarshalText parses an address in hex syntax.
func (a *Address) UnmarshalText(input []byte) error {
	buf := StringToBytes(string(input))
	if len(buf) != AddressLength {
		return fmt.Errorf("incorrect length")
	}

	*a = BytesToAddress(buf)

	return nil
}

func (a Address) MarshalText() ([]byte, error) {
	return []byte(a.String()), nil
}

// ImplementsGraphQLType returns true if Hash implements the specified GraphQL type.
func (a Address) ImplementsGraphQLType(name string) bool { return name == "Address" }

// UnmarshalGraphQL unmarshals the provided GraphQL query data.
func (a *Address) UnmarshalGraphQL(input interface{}) error {
	var err error

	switch input := input.(type) {
	case string:
		err = a.UnmarshalText([]byte(input))
	default:
		err = fmt.Errorf("unexpected type %T for Address", input)
	}

	return err
}
