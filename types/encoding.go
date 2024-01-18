package types

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/dogechain-lab/dogechain/helper/hex"
)

func ParseUint64orHex(val *string) (uint64, error) {
	if val == nil {
		return 0, nil
	}

	str := *val
	base := 10

	if strings.HasPrefix(str, hex.HexPrefix) {
		str = strings.TrimPrefix(str, hex.HexPrefix)
		base = 16
	}

	return strconv.ParseUint(str, base, 64)
}

func ParseUint256orHex(val *string) (*big.Int, error) {
	if val == nil {
		return nil, nil
	}

	str := *val
	base := 10

	if strings.HasPrefix(str, hex.HexPrefix) {
		str = str[2:]
		base = 16
	}

	b, ok := new(big.Int).SetString(str, base)

	if !ok {
		return nil, fmt.Errorf("could not parse")
	}

	return b, nil
}

func ParseInt64orHex(val *string) (int64, error) {
	i, err := ParseUint64orHex(val)

	return int64(i), err
}

func EncodeUint64(b uint64) *string {
	res := fmt.Sprintf(hex.HexPrefix+"%x", b)

	return &res
}

func ParseBytes(val *string) ([]byte, error) {
	if val == nil {
		return []byte{}, nil
	}

	str := strings.TrimPrefix(*val, hex.HexPrefix)

	return hex.DecodeString(str)
}

func EncodeBytes(b []byte) *string {
	if b == nil {
		return nil
	}

	res := hex.EncodeToHex(b)

	return &res
}

func EncodeBigInt(b *big.Int) *string {
	if b == nil {
		return nil
	}

	builder := new(strings.Builder)
	builder.Grow(b.BitLen()*2 + len(hex.HexPrefix))

	builder.WriteString(hex.HexPrefix)
	builder.WriteString(b.Text(16))

	res := builder.String()

	return &res
}
