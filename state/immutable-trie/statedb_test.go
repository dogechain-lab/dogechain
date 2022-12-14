package itrie

import (
	"testing"

	"github.com/dogechain-lab/dogechain/state"
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
