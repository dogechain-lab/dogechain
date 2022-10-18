package types

import (
	"math/big"
	"sync/atomic"

	"github.com/dogechain-lab/dogechain/helper/keccak"
)

type Transaction struct {
	Nonce    uint64
	GasPrice *big.Int
	Gas      uint64
	To       *Address
	Value    *big.Int
	Input    []byte
	V        *big.Int
	R        *big.Int
	S        *big.Int
	From     Address

	// Cache
	size atomic.Value
	hash atomic.Value
}

func (t *Transaction) IsContractCreation() bool {
	return t.To == nil
}

func (t *Transaction) Hash() Hash {
	if hash := t.hash.Load(); hash != nil {
		//nolint:forcetypeassert
		return hash.(Hash)
	}

	hash := t.rlpHash()
	t.hash.Store(hash)

	return hash
}

// rlpHash encodes transaction hash.
func (t *Transaction) rlpHash() (h Hash) {
	ar := marshalArenaPool.Get()
	hash := keccak.DefaultKeccakPool.Get()
	// return it back
	defer func() {
		keccak.DefaultKeccakPool.Put(hash)
		marshalArenaPool.Put(ar)
	}()

	v := t.MarshalRLPWith(ar)
	hash.WriteRlp(h[:0], v)

	return h
}

// Copy returns a deep copy
func (t *Transaction) Copy() *Transaction {
	tt := &Transaction{
		Nonce: t.Nonce,
		Gas:   t.Gas,
		From:  t.From,
	}

	tt.GasPrice = new(big.Int)
	if t.GasPrice != nil {
		tt.GasPrice.Set(t.GasPrice)
	}

	if t.To != nil {
		toAddr := *t.To
		tt.To = &toAddr
	}

	tt.Value = new(big.Int)
	if t.Value != nil {
		tt.Value.Set(t.Value)
	}

	if len(t.Input) > 0 {
		tt.Input = make([]byte, len(t.Input))
		copy(tt.Input[:], t.Input[:])
	}

	if t.V != nil {
		tt.V = new(big.Int).SetBits(t.V.Bits())
	}

	if t.R != nil {
		tt.R = new(big.Int).SetBits(t.R.Bits())
	}

	if t.S != nil {
		tt.S = new(big.Int).SetBits(t.S.Bits())
	}

	return tt
}

// Cost returns gas * gasPrice + value
func (t *Transaction) Cost() *big.Int {
	total := new(big.Int).Mul(t.GasPrice, new(big.Int).SetUint64(t.Gas))
	total.Add(total, t.Value)

	return total
}

func (t *Transaction) Size() uint64 {
	if size := t.size.Load(); size != nil {
		sizeVal, ok := size.(uint64)
		if !ok {
			return 0
		}

		return sizeVal
	}

	size := uint64(len(t.MarshalRLP()))
	t.size.Store(size)

	return size
}

func (t *Transaction) ExceedsBlockGasLimit(blockGasLimit uint64) bool {
	return t.Gas > blockGasLimit
}

func (t *Transaction) IsUnderpriced(priceLimit uint64) bool {
	return t.GasPrice.Cmp(big.NewInt(0).SetUint64(priceLimit)) < 0
}
