package types

import (
	"math/big"
	"reflect"
	"testing"
	"time"
)

func TestTransactionCopy(t *testing.T) {
	addrTo := StringToAddress("11")
	txn := &Transaction{
		Nonce:    0,
		GasPrice: big.NewInt(11),
		Gas:      11,
		To:       &addrTo,
		Value:    big.NewInt(1),
		Input:    []byte{1, 2},
		V:        big.NewInt(25),
		S:        big.NewInt(26),
		R:        big.NewInt(27),
	}
	newTxn := txn.Copy()

	if !reflect.DeepEqual(txn, newTxn) {
		t.Fatal("[ERROR] Copied transaction not equal base transaction")
	}
}

// Tests that if multiple transactions have the same price, the ones seen earlier
// are prioritized to avoid network spam attacks aiming for a specific ordering.
func TestTransactionTimeSort(t *testing.T) {
	addrs := []Address{
		StringToAddress("0x1"),
		StringToAddress("0x2"),
		StringToAddress("0x3"),
		StringToAddress("0x4"),
		StringToAddress("0x5"),
	}

	// Generate a batch of transactions with overlapping prices, but different creation times
	groups := map[Address][]*Transaction{}

	for start, addr := range addrs {
		// no sign, not matter in test
		tx := &Transaction{
			Nonce:        0,
			To:           &ZeroAddress,
			Value:        big.NewInt(100),
			Gas:          100,
			GasPrice:     big.NewInt(1),
			From:         addr,
			ReceivedTime: time.Unix(0, int64(len(addrs)-start)),
		}

		groups[addr] = append(groups[addr], tx)
	}
	// Sort the transactions and cross check the nonce ordering
	txset := NewTransactionsByPriceAndNonce(groups)

	txs := []*Transaction{}

	for tx := txset.Peek(); tx != nil; tx = txset.Peek() {
		txs = append(txs, tx)

		txset.Shift()
	}

	if len(txs) != len(addrs) {
		t.Errorf("expected %d transactions, found %d", len(addrs), len(txs))
	}

	for i, txi := range txs {
		fromi := txi.From

		if i+1 < len(txs) {
			next := txs[i+1]
			fromNext := next.From

			if txi.GasPrice.Cmp(next.GasPrice) < 0 {
				t.Errorf("invalid gasprice ordering: tx #%d (A=%x P=%v) < tx #%d (A=%x P=%v)",
					i, fromi[:4], txi.GasPrice, i+1, fromNext[:4], next.GasPrice)
			}
			// Make sure time order is ascending if the txs have the same gas price
			if txi.GasPrice.Cmp(next.GasPrice) == 0 && txi.ReceivedTime.After(next.ReceivedTime) {
				t.Errorf("invalid received time ordering: tx #%d (A=%x T=%v) > tx #%d (A=%x T=%v)",
					i, fromi[:4], txi.ReceivedTime, i+1, fromNext[:4], next.ReceivedTime)
			}
		}
	}
}
