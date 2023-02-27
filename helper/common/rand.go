package common

import (
	"crypto/rand"
	"math/big"
)

func SecureRandInt(max int) int {
	nBig, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		// if err not nil, it will panic, this is a low level error
		// program should not continue
		panic(err)
	}

	return ClampInt64ToInt(nBig.Int64())
}

func SecureRandInt64(max int64) int64 {
	nBig, err := rand.Int(rand.Reader, big.NewInt(max))
	if err != nil {
		// if err not nil, it will panic, this is a low level error
		// program should not continue
		panic(err)
	}

	return nBig.Int64()
}
