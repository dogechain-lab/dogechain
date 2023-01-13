package itrie

import (
	"math/rand"
	"testing"

	"github.com/dogechain-lab/dogechain/state"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/hashicorp/go-hclog"
)

func TestState(t *testing.T) {
	state.TestState(t, buildPreState)
}

func buildPreState(pre state.PreStates) (state.State, state.Snapshot) {
	storage := NewMemoryStorage()
	st := NewStateDB(storage, hclog.NewNullLogger(), nil)
	snap := st.NewSnapshot()

	return st, snap
}

func makeAccounts(size int) (addresses []types.Address, accounts []*state.PreState) {
	random := rand.New(rand.NewSource(0))

	addresses = make([]types.Address, size)
	accounts = make([]*state.PreState, size)

	for i := 0; i < len(addresses); i++ {
		data := make([]byte, types.AddressLength)
		random.Read(data)
		copy(addresses[i][:], data)
	}

	for i := 0; i < len(accounts); i++ {
		accounts[i] = &state.PreState{
			Nonce:   uint64(random.Int63()),
			Balance: uint64(random.Int63()),
		}
	}

	return addresses, accounts
}

func BenchmarkTxnCommit(b *testing.B) {
	addresses, accounts := makeAccounts(2 * b.N)

	storage := NewMemoryStorage()
	s := NewStateDB(storage, hclog.NewNullLogger(), nil)
	snap := s.NewSnapshot()

	txn := state.NewTxn(s, snap)
	for i := 0; i < len(addresses)/2; i++ {
		txn.SetNonce(addresses[i], accounts[i].Nonce+1)
		txn.SetState(addresses[i], types.BytesToHash(addresses[i][:]), types.BytesToHash([]byte{byte(i)}))
	}

	snap2, _, _ := snap.Commit(txn.Commit(false))

	txn = state.NewTxn(s, snap2)
	for i := len(addresses) / 2; i < len(addresses); i++ {
		txn.SetNonce(addresses[i], accounts[i].Nonce+1)
		txn.SetState(addresses[i], types.BytesToHash(addresses[i][:]), types.BytesToHash([]byte{byte(i)}))
	}

	b.ResetTimer()
	b.ReportAllocs()

	snap.Commit(txn.Commit(false))
}
