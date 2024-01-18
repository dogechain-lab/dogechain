package types

import (
	"math/big"
	"testing"

	"github.com/dogechain-lab/dogechain/helper/hex"
	"github.com/stretchr/testify/assert"
)

var (
	uint64Number = uint64(0x1234567890abcdef)
	uint64Zero   = uint64(0x0)
	uint64One    = uint64(0x1)

	uint64String  = "0x1234567890abcdef"
	uint64StrZero = "0x0"
	uint64StrOne  = "0x1"

	uint64ZeroLongLenVersion  = "0x0000000000000000000000000000000000000000000000000000000000000000"
	uint64OneLongLenVersion   = "0x0000000000000000000000000000000000000000000000000000000000000001"
	uint64ErrorLongLenVersion = "0x10000000000000000000000000000000000000000000000000000000000000000"

	uint256String = "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	uint256Zero   = "0x0"
	uint256One    = "0x1"
	uint256Error  = "0xG"

	uint256ZeroLongLenVersion = "0x0000000000000000000000000000000000000000000000000000000000000000"
	uint256OneLongLenVersion  = "0x0000000000000000000000000000000000000000000000000000000000000001"

	uint256Base10String = "123456789123456789123456789123456789123456789123456789123456789"
	uint256Base10Zero   = "0"
	uint256Base10One    = "1"
	uint256Base10Error  = "F"

	uint256Base10ZeroLongLenVersion = "0000000000000000000000000000000000000000000000000000000000000000"
	uint256Base10OneLongLenVersion  = "0000000000000000000000000000000000000000000000000000000000000001"

	bytesString      = "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	bytesStringError = "0xG"

	bytesStringBytes = []byte{0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef, 0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef, 0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef, 0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef}
)

func TestParseUint64orHex(t *testing.T) {
	v, err := ParseUint64orHex(nil)
	assert.Nil(t, err)
	assert.Equal(t, v, uint64(0))

	v, err = ParseUint64orHex(&uint64String)
	assert.Nil(t, err)
	assert.Equal(t, v, uint64(0x1234567890abcdef))

	v, err = ParseUint64orHex(&uint64StrZero)
	assert.Nil(t, err)
	assert.Equal(t, v, uint64(0))

	v, err = ParseUint64orHex(&uint64StrOne)
	assert.Nil(t, err)
	assert.Equal(t, v, uint64(1))

	v, err = ParseUint64orHex(&uint64ZeroLongLenVersion)
	assert.Nil(t, err)
	assert.Equal(t, v, uint64(0))

	v, err = ParseUint64orHex(&uint64OneLongLenVersion)
	assert.Nil(t, err)
	assert.Equal(t, v, uint64(1))

	_, err = ParseUint64orHex(&uint64ErrorLongLenVersion)
	assert.NotNil(t, err)
}

func TestParseUint256orHex(t *testing.T) {
	v, err := ParseUint256orHex(nil)
	assert.Nil(t, err)
	assert.Nil(t, v)

	_, err = ParseUint256orHex(&uint256Error)
	assert.NotNil(t, err)

	v, err = ParseUint256orHex(&uint256String)
	assert.Nil(t, err)
	assert.Equal(t, hex.HexPrefix+v.Text(16), uint256String)

	v, err = ParseUint256orHex(&uint256Zero)
	assert.Nil(t, err)
	assert.Equal(t, hex.HexPrefix+v.String(), uint256Zero)

	v, err = ParseUint256orHex(&uint256One)
	assert.Nil(t, err)
	assert.Equal(t, hex.HexPrefix+v.String(), uint256One)

	v, err = ParseUint256orHex(&uint256ZeroLongLenVersion)
	assert.Nil(t, err)
	assert.Equal(t, hex.HexPrefix+v.String(), uint256Zero)

	v, err = ParseUint256orHex(&uint256OneLongLenVersion)
	assert.Nil(t, err)
	assert.Equal(t, hex.HexPrefix+v.String(), uint256One)

	// Base 10 string tests
	_, err = ParseUint256orHex(&uint256Base10Error)
	assert.NotNil(t, err)

	v, err = ParseUint256orHex(&uint256Base10String)
	assert.Nil(t, err)
	assert.Equal(t, v.Text(10), uint256Base10String)

	v, err = ParseUint256orHex(&uint256Base10Zero)
	assert.Nil(t, err)
	assert.Equal(t, v.Text(10), uint256Base10Zero)

	v, err = ParseUint256orHex(&uint256Base10One)
	assert.Nil(t, err)
	assert.Equal(t, v.Text(10), uint256Base10One)

	v, err = ParseUint256orHex(&uint256Base10ZeroLongLenVersion)
	assert.Nil(t, err)
	assert.Equal(t, v.Text(10), uint256Base10Zero)

	v, err = ParseUint256orHex(&uint256Base10OneLongLenVersion)
	assert.Nil(t, err)
	assert.Equal(t, v.Text(10), uint256Base10One)
}

func TestInt64orHex(t *testing.T) {
	v, err := ParseInt64orHex(nil)
	assert.Nil(t, err)
	assert.Equal(t, v, int64(0))

	v, err = ParseInt64orHex(EncodeUint64(uint64Number))
	assert.Nil(t, err)
	assert.Equal(t, v, int64(0x1234567890abcdef))

	v, err = ParseInt64orHex(EncodeUint64(uint64Zero))
	assert.Nil(t, err)
	assert.Equal(t, v, int64(0))

	v, err = ParseInt64orHex(EncodeUint64(uint64One))
	assert.Nil(t, err)
	assert.Equal(t, v, int64(1))

	v, err = ParseInt64orHex(&uint64OneLongLenVersion)
	assert.Nil(t, err)
	assert.Equal(t, v, int64(1))

	_, err = ParseInt64orHex(&uint64ErrorLongLenVersion)
	assert.NotNil(t, err)
}

func TestBytesParseEncode(t *testing.T) {
	{
		// Test parse
		v, err := ParseBytes(nil)
		assert.Nil(t, err)
		assert.Equal(t, v, []byte{})

		v, err = ParseBytes(&bytesString)
		assert.Nil(t, err)
		assert.Equal(t, v, bytesStringBytes)

		_, err = ParseBytes(&bytesStringError)
		assert.NotNil(t, err)
	}

	{
		// Test encode
		v := EncodeBytes(nil)
		assert.Nil(t, v)

		v = EncodeBytes(bytesStringBytes)
		assert.Equal(t, *v, bytesString)
	}
}

func TestEncodeBigInt(t *testing.T) {
	v := EncodeBigInt(nil)
	assert.Nil(t, v)

	v = EncodeBigInt(big.NewInt(0))
	assert.Equal(t, *v, "0x0")

	v = EncodeBigInt(big.NewInt(1))
	assert.Equal(t, *v, "0x1")

	v = EncodeBigInt(big.NewInt(0x1234567890abcdef))
	assert.Equal(t, *v, "0x1234567890abcdef")
}
