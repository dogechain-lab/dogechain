package currentstate

import (
	"testing"
	"time"

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
	// fake addr
	var addr1 = types.StringToAddress("1")

	testCases := []struct {
		description string
		c           *CurrentState
		expected    time.Duration
	}{
		{
			description: "for 0 validator returns 10s",
			c: &CurrentState{
				validators: validator.Validators{},
			},
			expected: baseTimeout,
		},
		{
			description: "for 1 validator returns 10s",
			c: &CurrentState{
				validators: validator.Validators{
					addr1,
				}},
			expected: baseTimeout,
		},
		{
			description: "for 2 validators returns 10s",
			c: &CurrentState{
				validators: validator.Validators{
					addr1, addr1,
				}},
			expected: baseTimeout,
		},
		{
			description: "for 3 validators returns 12s",
			c: &CurrentState{
				validators: validator.Validators{
					addr1, addr1, addr1,
				}},
			expected: baseTimeout + 2*time.Second,
		},
		{
			description: "for 13 validators returns 18s",
			c: &CurrentState{
				validators: validator.Validators{
					addr1, addr1, addr1, addr1, addr1, addr1, addr1,
					addr1, addr1, addr1, addr1, addr1, addr1,
				}},
			expected: baseTimeout + 8*time.Second,
		},
		{
			description: "for 23 validators returns 24s",
			c: &CurrentState{
				validators: validator.Validators{
					addr1, addr1, addr1, addr1, addr1, addr1, addr1,
					addr1, addr1, addr1, addr1, addr1, addr1, addr1,
					addr1, addr1, addr1, addr1, addr1, addr1, addr1,
					addr1, addr1,
				}},
			expected: baseTimeout + 14*time.Second,
		},
		{
			description: "for 24 validators returns 26s",
			c: &CurrentState{
				validators: validator.Validators{
					addr1, addr1, addr1, addr1, addr1, addr1, addr1,
					addr1, addr1, addr1, addr1, addr1, addr1, addr1,
					addr1, addr1, addr1, addr1, addr1, addr1, addr1,
					addr1, addr1, addr1,
				}},
			expected: baseTimeout + 16*time.Second,
		},
		{
			description: "for 28 validators returns 26s",
			c: &CurrentState{
				validators: validator.Validators{
					addr1, addr1, addr1, addr1, addr1, addr1, addr1,
					addr1, addr1, addr1, addr1, addr1, addr1, addr1,
					addr1, addr1, addr1, addr1, addr1, addr1, addr1,
					addr1, addr1, addr1, addr1, addr1, addr1, addr1,
				}},
			expected: baseTimeout + 16*time.Second,
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
