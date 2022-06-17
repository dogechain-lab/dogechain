package types

import (
	"database/sql/driver"
	"errors"
	"fmt"

	"github.com/dogechain-lab/dogechain/helper/hex"
)

var (
	// ZeroHash is all zero hash
	ZeroHash = Hash{}

	// EmptyRootHash is the root when there are no transactions
	EmptyRootHash = StringToHash("0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421")

	// EmptyUncleHash is the root when there are no uncles
	EmptyUncleHash = StringToHash("0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347")
)

const (
	HashLength = 32
)

type Hash [HashLength]byte

func StringToHash(str string) Hash {
	return BytesToHash(stringToBytes(str))
}

func BytesToHash(b []byte) Hash {
	var h Hash

	size := len(b)
	min := min(size, HashLength)

	copy(h[HashLength-min:], b[len(b)-min:])

	return h
}

func (h Hash) Bytes() []byte {
	return h[:]
}

func (h Hash) String() string {
	return hex.EncodeToHex(h[:])
}

func (h Hash) Value() (driver.Value, error) {
	return h.String(), nil
}

func (h *Hash) Scan(src interface{}) error {
	stringVal, ok := src.([]byte)
	if !ok {
		return errors.New("invalid type assert")
	}

	hh, err := hex.DecodeHex(string(stringVal))
	if err != nil {
		return fmt.Errorf("decode hex err: %w", err)
	}

	copy(h[:], hh[:])

	return nil
}

// UnmarshalText parses a hash in hex syntax.
func (h *Hash) UnmarshalText(input []byte) error {
	*h = BytesToHash(stringToBytes(string(input)))

	return nil
}

func (h Hash) MarshalText() ([]byte, error) {
	return []byte(h.String()), nil
}

// ImplementsGraphQLType returns true if Hash implements the specified GraphQL type.
func (h Hash) ImplementsGraphQLType(name string) bool { return name == "Bytes32" }

// UnmarshalGraphQL unmarshals the provided GraphQL query data.
func (h *Hash) UnmarshalGraphQL(input interface{}) error {
	var err error

	switch input := input.(type) {
	case string:
		err = h.UnmarshalText([]byte(input))
	default:
		err = fmt.Errorf("unexpected type %T for Hash", input)
	}

	return err
}
