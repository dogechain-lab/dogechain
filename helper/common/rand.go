package common

import (
	"crypto/rand"
	"log"
	"math/big"
)

func SecureRandInt(max int) int {
	nBig, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		log.Println(err)
	}

	return ClampInt64ToInt(nBig.Int64())
}

func SecureRandInt64(max int64) int64 {
	nBig, err := rand.Int(rand.Reader, big.NewInt(max))
	if err != nil {
		log.Println(err)
	}

	return nBig.Int64()
}
