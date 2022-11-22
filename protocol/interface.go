package protocol

import (
	"github.com/dogechain-lab/dogechain/blockchain"
	"github.com/dogechain-lab/dogechain/helper/progress"
)

type Progression interface {
	// StartProgression starts progression
	StartProgression(startingBlock uint64, subscription blockchain.Subscription)
	// UpdateHighestProgression updates highest block number
	UpdateHighestProgression(highestBlock uint64)
	// GetProgression returns Progression
	GetProgression() *progress.Progression
	// StopProgression finishes progression
	StopProgression()
}
