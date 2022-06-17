package types

import (
	"strings"

	"github.com/dogechain-lab/dogechain/helper/hex"
)

func min(i, j int) int {
	if i < j {
		return i
	}

	return j
}

func stringToBytes(str string) []byte {
	str = strings.TrimPrefix(str, "0x")
	if len(str)%2 == 1 {
		str = "0" + str
	}

	b, _ := hex.DecodeString(str)

	return b
}
