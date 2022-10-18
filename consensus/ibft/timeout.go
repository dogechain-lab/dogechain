package ibft

import (
	"time"
)

const (
	baseTimeout = 4 * time.Second
	maxTimeout  = 20 * time.Second
)

// messageTimeout returns duration for waiting message
//
// Consider the network travel time between most validators, using validator
// numbers instead of rounds.
func (i *Ibft) messageTimeout() time.Duration {
	if i.state == nil || len(i.state.validators) == 0 {
		return baseTimeout
	}

	validatorNumbers := len(i.state.validators)
	if validatorNumbers >= 24 {
		return maxTimeout
	}

	// 2 second steps
	return baseTimeout + time.Duration(int(validatorNumbers/3)*2)*time.Second
}
