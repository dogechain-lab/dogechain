package types

import (
	"math/big"
	"sync/atomic"
	"time"

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
	Hash     Hash
	From     Address

	// Cache
	size atomic.Value

	// time at which the node received the tx
	ReceivedTime time.Time
}

func (t *Transaction) IsContractCreation() bool {
	return t.To == nil
}

// ComputeHash computes the hash of the transaction
func (t *Transaction) ComputeHash() *Transaction {
	ar := marshalArenaPool.Get()
	hash := keccak.DefaultKeccakPool.Get()

	v := t.MarshalRLPWith(ar)
	hash.WriteRlp(t.Hash[:0], v)

	marshalArenaPool.Put(ar)
	keccak.DefaultKeccakPool.Put(hash)

	return t
}

// Copy returns a deep copy
func (t *Transaction) Copy() *Transaction {
	tt := &Transaction{
		Nonce: t.Nonce,
		Gas:   t.Gas,
		Hash:  t.Hash,
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

	tt.ReceivedTime = t.ReceivedTime

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

// TxWithMinerFee wraps a transaction with its gas price or effective miner gasTipCap
type TxWithMinerFee struct {
	tx       *Transaction
	minerFee *big.Int
}

// TxByPriceAndTime implements both the sort and the heap interface, making it useful
// for all at once sorting as well as individually adding and removing elements.
type TxByPriceAndTime []*TxWithMinerFee

func (s TxByPriceAndTime) Len() int {
	return len(s)
}

func (s TxByPriceAndTime) Less(i, j int) bool {
	// If the prices are equal, use the time the transaction was first seen for deterministic sorting
	cmp := s[i].minerFee.Cmp(s[j].minerFee)
	if cmp == 0 {
		return s[i].tx.ReceivedTime.Before(s[j].tx.ReceivedTime)
	}

	return cmp > 0
}

func (s TxByPriceAndTime) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s *TxByPriceAndTime) Push(x interface{}) {
	*s = append(*s, x.(*TxWithMinerFee))
}

func (s *TxByPriceAndTime) Pop() interface{} {
	old := *s
	n := len(old)
	x := old[n-1]
	*s = old[0 : n-1]

	return x
}
