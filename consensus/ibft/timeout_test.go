package ibft

import (
	"testing"
	"time"

	"github.com/dogechain-lab/dogechain/types"
	"github.com/stretchr/testify/assert"
)

func TestExponentialTimeout(t *testing.T) {
	// fake addr
	var addr1 = types.StringToAddress("1")

	//nolint:lll
	testCases := []struct {
		description string
		i           *Ibft
		expected    time.Duration
	}{
		{
			description: "for 0 validator returns 4s",
			i: &Ibft{state: &currentState{
				validators: ValidatorSet{},
			}},
			expected: 4 * time.Second,
		},
		{
			description: "for 1 validator returns 4s",
			i: &Ibft{state: &currentState{
				validators: ValidatorSet{
					addr1,
				}}},
			expected: 4 * time.Second,
		},
		{
			description: "for 2 validators returns 4s",
			i: &Ibft{state: &currentState{
				validators: ValidatorSet{
					addr1, addr1,
				}}},
			expected: 4 * time.Second,
		},
		{
			description: "for 3 validators returns 6s",
			i: &Ibft{state: &currentState{
				validators: ValidatorSet{
					addr1, addr1, addr1,
				}}},
			expected: 6 * time.Second,
		},
		{
			description: "for 13 validators returns 12s",
			i: &Ibft{state: &currentState{
				validators: ValidatorSet{
					addr1, addr1, addr1, addr1, addr1, addr1, addr1,
					addr1, addr1, addr1, addr1, addr1, addr1,
				}}},
			expected: 12 * time.Second,
		},
		{
			description: "for 23 validators returns 18s",
			i: &Ibft{state: &currentState{
				validators: ValidatorSet{
					addr1, addr1, addr1, addr1, addr1, addr1, addr1,
					addr1, addr1, addr1, addr1, addr1, addr1, addr1,
					addr1, addr1, addr1, addr1, addr1, addr1, addr1,
					addr1, addr1,
				}}},
			expected: 18 * time.Second,
		},
		{
			description: "for 24 validators returns 20s",
			i: &Ibft{state: &currentState{
				validators: ValidatorSet{
					addr1, addr1, addr1, addr1, addr1, addr1, addr1,
					addr1, addr1, addr1, addr1, addr1, addr1, addr1,
					addr1, addr1, addr1, addr1, addr1, addr1, addr1,
					addr1, addr1, addr1,
				}}},
			expected: 20 * time.Second,
		},
		{
			description: "for 28 validators returns 20s",
			i: &Ibft{state: &currentState{
				validators: ValidatorSet{
					addr1, addr1, addr1, addr1, addr1, addr1, addr1,
					addr1, addr1, addr1, addr1, addr1, addr1, addr1,
					addr1, addr1, addr1, addr1, addr1, addr1, addr1,
					addr1, addr1, addr1, addr1, addr1, addr1, addr1,
				}}},
			expected: 20 * time.Second,
		},
	}

	for _, test := range testCases {
		test := test
		t.Run(test.description, func(t *testing.T) {
			timeout := test.i.messageTimeout()

			assert.Equal(t, test.expected, timeout)
		})
	}
}
