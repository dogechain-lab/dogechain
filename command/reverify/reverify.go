package reverify

import (
	"fmt"
	"path/filepath"

	"github.com/dogechain-lab/dogechain/command"
	"github.com/dogechain-lab/dogechain/command/helper"
	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"

	itrie "github.com/dogechain-lab/dogechain/state/immutable-trie"
)

func GetCommand() *cobra.Command {
	reverifyCmd := &cobra.Command{
		Use:     "reverify",
		Short:   "Reverify block data",
		PreRunE: runPreRun,
		Run:     runCommand,
	}

	helper.RegisterPprofFlag(reverifyCmd)
	helper.SetRequiredFlags(reverifyCmd, params.getRequiredFlags())

	setFlags(reverifyCmd)

	return reverifyCmd
}

func setFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&params.DataDir,
		dataDirFlag,
		"",
		"the data directory used for storing Dogechain-Lab Dogechain client data",
	)

	cmd.Flags().StringVar(
		&params.startHeightRaw,
		startHeight,
		"1",
		"start reverify block height",
	)

	cmd.Flags().StringVar(
		&params.GenesisPath,
		genesisPath,
		"./genesis.json",
		"the genesis file path",
	)
}

func runPreRun(cmd *cobra.Command, args []string) error {
	return params.validateFlags()
}

func runCommand(cmd *cobra.Command, _ []string) {
	command.InitializePprofServer(cmd)

	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "reverify",
		Level: hclog.Info,
	})

	verifyStartHeight := params.startHeight
	if verifyStartHeight <= 0 {
		logger.Error("verify height must be greater than 0")

		return
	}

	chain, err := parseGenesis(params.GenesisPath)
	if err != nil {
		logger.Error("failed to parse genesis", "err", err)

		return
	}

	stateStorage, err := itrie.NewLevelDBStorage(
		newLevelDBBuilder(logger, filepath.Join(params.DataDir, "trie")))
	if err != nil {
		logger.Error("failed to create state storage", "err", err)

		return
	}
	defer stateStorage.Close()

	blockchain, consensus, err := createBlockchain(
		logger,
		chain,
		itrie.NewState(stateStorage, nil),
		params.DataDir,
	)
	if err != nil {
		logger.Error("failed to create blockchain", "err", err)

		return
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

	for i := verifyStartHeight; i <= currentHeight; i++ {
		haeder, ok := blockchain.GetHeaderByNumber(i)
		if !ok {
			logger.Error("failed to read canonical hash", "height", i, "header", haeder)

			return
		}

		block, ok := blockchain.GetBlock(haeder.Hash, i, true)
		if !ok {
			logger.Error("failed to read block", "hash", haeder)

			return
		}

		if err := blockchain.VerifyFinalizedBlock(block); err != nil {
			logger.Error("failed to verify block", "hash", haeder, "err", err)

			return
		}

		logger.Info("verify block success", "height", i, "hash", haeder.Hash, "txs", len(block.Transactions))
	}

	logger.Info(fmt.Sprintf("verify height from %d to %d \n", params.startHeight, currentHeight))
}
