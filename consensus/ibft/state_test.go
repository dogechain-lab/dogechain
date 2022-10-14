package ibft

import (
	"testing"

	"github.com/dogechain-lab/dogechain/consensus/ibft/proto"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/stretchr/testify/assert"
)

func TestState_FaultyNodes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		Network, Faulty uint64
	}{
		{1, 0},
		{2, 0},
		{3, 0},
		{4, 1},
		{5, 1},
		{6, 1},
		{7, 2},
		{8, 2},
		{9, 2},
	}
	for _, c := range cases {
		pool := newTesterAccountPool(int(c.Network))
		vals := pool.ValidatorSet()
		assert.Equal(t, vals.MaxFaultyNodes(), int(c.Faulty))
	}
}

func TestState_AddMessages(t *testing.T) {
	t.Parallel()

	pool := newTesterAccountPool()
	pool.add("A", "B", "C", "D")

	c := newState()
	c.validators = pool.ValidatorSet()

	msg := func(acct string, typ proto.MessageReq_Type, round ...uint64) *proto.MessageReq {
		msg := &proto.MessageReq{
			From: pool.get(acct).Address().String(),
			Type: typ,
			View: &proto.View{Round: 0},
		}
		r := uint64(0)

		if len(round) > 0 {
			r = round[0]
		}

		msg.View.Round = r

		return msg
	}

	// -- test committed messages --
	c.addMessage(msg("A", proto.MessageReq_Commit))
	c.addMessage(msg("B", proto.MessageReq_Commit))
	c.addMessage(msg("B", proto.MessageReq_Commit))

	assert.Equal(t, c.numCommitted(), 2)

	// -- test prepare messages --
	c.addMessage(msg("C", proto.MessageReq_Prepare))
	c.addMessage(msg("C", proto.MessageReq_Prepare))
	c.addMessage(msg("D", proto.MessageReq_Prepare))

	assert.Equal(t, c.numPrepared(), 2)
}

func TestState_PorposerAndNeedPunished(t *testing.T) {
	t.Parallel()

	var (
		v1 = types.StringToAddress("0x1")
		v2 = types.StringToAddress("0x2")
		v3 = types.StringToAddress("0x3")
		v4 = types.StringToAddress("0x4")
	)

	state := newState()
	state.validators = ValidatorSet{v1, v2, v3, v4}

	tests := []struct {
		name              string
		round             uint64
		lastBlockProposer types.Address
		supporseProposer  types.Address
		needPunished      []types.Address
	}{
		{
			name:              "round 0 should not punish anyone",
			round:             0,
			lastBlockProposer: v1,
			supporseProposer:  v2,
			needPunished:      nil,
		},
		{
			name:              "round 2 should punish first validator",
			round:             2,
			lastBlockProposer: v3,
			supporseProposer:  v2,
			needPunished:      []types.Address{v4},
		},
		{
			name:              "large round should punish first validator",
			round:             9,
			lastBlockProposer: v2,
			supporseProposer:  v4,
			needPunished:      []types.Address{v3},
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			proposer := state.validators.CalcProposer(tt.round, tt.lastBlockProposer)
			assert.Equal(t, tt.supporseProposer, proposer)

			punished := state.CalcNeedPunished(tt.round, tt.lastBlockProposer)
			assert.Equal(t, tt.needPunished, punished)
		})
	}
}
