package currentstate

import (
	"testing"
	"time"

	"github.com/dogechain-lab/dogechain/consensus/ibft/proto"
	"github.com/dogechain-lab/dogechain/consensus/ibft/validator"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/stretchr/testify/assert"
)

func TestState_PorposerAndNeedPunished(t *testing.T) {
	t.Parallel()

	var (
		v1 = types.StringToAddress("0x1")
		v2 = types.StringToAddress("0x2")
		v3 = types.StringToAddress("0x3")
		v4 = types.StringToAddress("0x4")
	)

	state := NewState()
	state.validators = validator.Validators{v1, v2, v3, v4}

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

func TestState_MessageTimeout(t *testing.T) {
	testCases := []struct {
		description string
		c           *CurrentState
		expected    time.Duration
	}{
		{
			description: "round 0 return 10s",
			c:           &CurrentState{view: proto.ViewMsg(1, 0)},
			expected:    baseTimeout,
		},
		{
			description: "round 1 returns 12s",
			c:           &CurrentState{view: proto.ViewMsg(1, 1)},
			expected:    baseTimeout + 2*time.Second,
		},
		{
			description: "round 3 returns 18s",
			c:           &CurrentState{view: proto.ViewMsg(1, 3)},
			expected:    baseTimeout + 8*time.Second,
		},
		{
			description: "round 7 returns 138s",
			c:           &CurrentState{view: proto.ViewMsg(1, 7)},
			expected:    baseTimeout + 128*time.Second,
		},
		{
			description: "round 8 returns 300s",
			c:           &CurrentState{view: proto.ViewMsg(1, 8)},
			expected:    maxTimeout,
		},
		{
			description: "round 9 returns 300s",
			c:           &CurrentState{view: proto.ViewMsg(1, 9)},
			expected:    maxTimeout,
		},
	}

	for _, test := range testCases {
		test := test
		t.Run(test.description, func(t *testing.T) {
			timeout := test.c.MessageTimeout()

			assert.Equal(t, test.expected, timeout)
		})
	}
}
