package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEIP55(t *testing.T) {
	cases := []struct {
		address  string
		expected string
	}{
		{
			"0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed",
			"0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
		},
		{
			"0xfb6916095ca1df60bb79ce92ce3ea74c37c5d359",
			"0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359",
		},
		{
			"0xdbf03b407c01e7cd3cbea99509d93f8dddc8c6fb",
			"0xdbF03B407c01E7cD3CBea99509d93f8DDDC8C6FB",
		},
		{
			"0xd1220a0cf47c7b9be7a2e6ba89f429762e7b9adb",
			"0xD1220A0cf47c7B9Be7A2E6BA89F429762e7b9aDb",
		},
		{
			"0xde64a66c41599905950ca513fa432187a8c65679",
			"0xde64A66C41599905950ca513Fa432187a8C65679",
		},
		{
			"0xb41364957365228984ea8ee98e80dbed4b9ffcdc",
			"0xB41364957365228984eA8EE98e80DBED4B9fFcDC",
		},
		{
			"0xb529594951753de833b00865d7fe52cc4d8b0f63",
			"0xB529594951753DE833b00865D7FE52cC4d8B0f63",
		},
		{
			"0xb529594951753de833b00865",
			"0x0000000000000000B529594951753De833B00865",
		},
		{
			"0xeEd210D",
			"0x000000000000000000000000000000000eED210d",
		},
	}

	for _, c := range cases {
		t.Run("", func(t *testing.T) {
			addr := StringToAddress(c.address)
			assert.Equal(t, c.expected, addr.String())
		})
	}
}

func TestAddressToString(t *testing.T) {
	cases := []struct {
		address  Address
		expected string
	}{
		{
			[20]byte{0x5a, 0xae, 0xb6, 0x05, 0x3f, 0x3e, 0x94, 0xc9, 0xb9, 0xa0, 0x9f, 0x33, 0x66, 0x94, 0x35, 0xe7, 0xef, 0x1b, 0xea, 0xed},
			"0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
		},
		{
			[20]byte{0xfb, 0x69, 0x16, 0x09, 0x5c, 0xa1, 0xdf, 0x60, 0xbb, 0x79, 0xce, 0x92, 0xce, 0x3e, 0xa7, 0x4c, 0x37, 0xc5, 0xd3, 0x59},
			"0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359",
		},
		{
			[20]byte{0xdb, 0xf0, 0x3b, 0x40, 0x7c, 0x01, 0xe7, 0xcd, 0x3c, 0xbe, 0xa9, 0x95, 0x09, 0xd9, 0x3f, 0x8d, 0xdd, 0xc8, 0xc6, 0xfb},
			"0xdbF03B407c01E7cD3CBea99509d93f8DDDC8C6FB",
		},
	}

	for _, c := range cases {
		t.Run("", func(t *testing.T) {
			assert.Equal(t, c.expected, c.address.String())
		})
	}
}

func BenchmarkEIP55(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = StringToAddress("0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed").String()
		}
	})
}
