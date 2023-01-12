package hex

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

const (
	HexPrefix = "0x"
	HexZero   = HexPrefix + "0"
	HexOne    = HexPrefix + "1"
)

// EncodeToHex generates a hex string based on the byte representation, with the '0x' prefix
func EncodeToHex(str []byte) string {
	builder := new(strings.Builder)
	builder.Grow(len(str)*2 + len(HexPrefix))

	builder.WriteString(HexPrefix)
	builder.WriteString(hex.EncodeToString(str))

	return builder.String()
}

// EncodeToString is a wrapper method for hex.EncodeToString
func EncodeToString(str []byte) string {
	return hex.EncodeToString(str)
}

// DecodeString returns the byte representation of the hexadecimal string
func DecodeString(str string) ([]byte, error) {
	return hex.DecodeString(str)
}

// DecodeHex converts a hex string to a byte array
func DecodeHex(str string) ([]byte, error) {
	str = strings.TrimPrefix(str, HexPrefix)

	return hex.DecodeString(str)
}

// EncodeUint64 encodes a number as a hex string with 0x prefix.
func EncodeUint64(i uint64) string {
	builder := new(strings.Builder)
	builder.Grow(16 + len(HexPrefix))

	builder.WriteString(HexPrefix)
	builder.WriteString(strconv.FormatUint(i, 16))

	return builder.String()
}

// EncodeBig encodes bigint as a hex string with 0x prefix.
// The sign of the integer is ignored.
func EncodeBig(bigint *big.Int) string {
	if bigint.BitLen() == 0 {
		return HexZero
	}

	return fmt.Sprintf("%#x", bigint)
}
