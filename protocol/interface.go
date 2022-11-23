package protocol

import (
	"math/big"

	"github.com/dogechain-lab/dogechain/blockchain"
	"github.com/dogechain-lab/dogechain/helper/progress"
	"github.com/dogechain-lab/dogechain/types"
)

// Blockchain is the interface required by the syncer to connect to the blockchain
type Blockchain interface {
	// SubscribeEvents subscribes new blockchain event
	SubscribeEvents() blockchain.Subscription
	Header() *types.Header

	// deprecated methods. Those are old version protocols, keep it only for backward compatible
	CurrentTD() *big.Int
	GetTD(hash types.Hash) (*big.Int, bool)
	GetReceiptsByHash(types.Hash) ([]*types.Receipt, error)
	GetBodyByHash(types.Hash) (*types.Body, bool)
	GetHeaderByHash(types.Hash) (*types.Header, bool)
	GetHeaderByNumber(n uint64) (*types.Header, bool)
	CalculateGasLimit(number uint64) (uint64, error)

	// advance chain methods
	WriteBlock(block *types.Block) error
	VerifyFinalizedBlock(block *types.Block) error
}

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
