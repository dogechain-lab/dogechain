package hex

import (
	"encoding/hex"
	"math/big"
	"strconv"
	"testing"
)

var (
	testEncodeToHexBytes = [][]byte{
		[]byte("1EncodeToHex1"),
		[]byte("2EncodeToHex2"),
	}

	testEncodeToHexResult = []string{
		"0x31456e636f6465546f48657831",
		"0x32456e636f6465546f48657832",
	}

	testDecodeHexStrings = []string{
		"0x314465636f6465537472696e6731",
		"0x324465636f6465537472696e6732",
	}

	testDecodeHexResults = []string{
		"1DecodeString1",
		"2DecodeString2",
	}
)

func TestEncodeToHex(t *testing.T) {
	for i := 0; i < len(testEncodeToHexBytes); i++ {
		if EncodeToHex(testEncodeToHexBytes[i]) != testEncodeToHexResult[i] {
			t.Error("EncodeToHex failed")
		}
	}
}

func TestDecodeHex(t *testing.T) {
	for i := 0; i < len(testDecodeHexStrings); i++ {
		if s, ok := DecodeHex(testDecodeHexStrings[i]); ok != nil || string(s) != testDecodeHexResults[i] {
			t.Error("DecodeString failed")
		}
	}
}

func TestEncodeUint64(t *testing.T) {
	if EncodeUint64(0) != HexZero {
		t.Error("EncodeUint64 failed")
	}

	if EncodeUint64(1) != HexOne {
		t.Error("EncodeUint64 failed")
	}

	if EncodeUint64(0x1234567890abcdef) != "0x1234567890abcdef" {
		t.Error("EncodeUint64 failed")
	}
}

func TestEncodeBig(t *testing.T) {
	if EncodeBig(big.NewInt(0)) != HexZero {
		t.Error("EncodeBig failed")
	}

	if EncodeBig(big.NewInt(1)) != HexOne {
		t.Error("EncodeBig failed")
	}

	if EncodeBig(big.NewInt(0x1234567890abcdef)) != "0x1234567890abcdef" {
		t.Error("EncodeBig failed")
	}
}

// EncodeToHex benchmark
func BenchmarkEncodeToHex(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			EncodeToHex([]byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))
		}
	})
}

// EncodeToHex (use `+`) benchmark
func BenchmarkEncodeToHexUsePlus(b *testing.B) {
	plus := func(str []byte) string {
		return HexPrefix + hex.EncodeToString(str)
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			plus([]byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))
		}
	})
}

func BenchmarkEncodeUint64(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			EncodeUint64(0x1234567890abcdef)
		}
	})
}

func BenchmarkEncodeUint64Old(b *testing.B) {
	old := func(i uint64) string {
		enc := make([]byte, 2, 16)
		copy(enc, []byte(HexPrefix))

		return string(strconv.AppendUint(enc, i, 16))
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			old(0x1234567890abcdef)
		}
	})
}
