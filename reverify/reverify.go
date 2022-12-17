package reverify

import (
	"fmt"
	"path/filepath"

	"github.com/hashicorp/go-hclog"

	"github.com/dogechain-lab/dogechain/chain"
	itrie "github.com/dogechain-lab/dogechain/state/immutable-trie"
)

func ReverifyChain(
	logger hclog.Logger,
	chain *chain.Chain,
	dataDir string,
	startHeight uint64,
) error {
	stateStorage, err := itrie.NewLevelDBStorage(
		newLevelDBBuilder(logger, filepath.Join(dataDir, "trie")))
	if err != nil {
		logger.Error("failed to create state storage")

		return err
	}
	defer stateStorage.Close()

	blockchain, consensus, err := createBlockchain(
		logger,
		chain,
		itrie.NewStateDB(stateStorage, hclog.NewNullLogger(), itrie.NilMetrics()),
		dataDir,
	)
	if err != nil {
		logger.Error("failed to create blockchain")

		return err
	}
	defer blockchain.Close()
	defer consensus.Close()

	hash, ok := blockchain.GetHeaderHash()
	if ok {
		logger.Info("current blockchain hash", "hash", hash)
	}

	currentHeight, ok := blockchain.GetHeaderNumber()
	if ok {
		logger.Info("current blockchain height", "Height", currentHeight)
	}

	for i := startHeight; i <= currentHeight; i++ {
		haeder, ok := blockchain.GetHeaderByNumber(i)
		if !ok {
			return fmt.Errorf("failed to read canonical hash, height: %d, header: %v", i, haeder)
		}

		block, ok := blockchain.GetBlock(haeder.Hash, i, true)
		if !ok {
			return fmt.Errorf("failed to read block, height: %d, header: %v", i, haeder)
		}

		if err := blockchain.VerifyFinalizedBlock(block); err != nil {
			return fmt.Errorf("failed to verify block, height: %d, header: %v", i, haeder)
		}

		logger.Info("verify block success", "height", i, "hash", haeder.Hash, "txs", len(block.Transactions))
	}

	logger.Info(fmt.Sprintf("verify height from %d to %d \n", startHeight, currentHeight))

	return nil
}
