package reverify

import (
	"fmt"

	"github.com/dogechain-lab/dogechain/chain"
	"github.com/dogechain-lab/dogechain/command"
	"github.com/dogechain-lab/dogechain/command/helper"
	"github.com/dogechain-lab/dogechain/reverify"
	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"
)

func parseGenesis(genesisPath string) (*chain.Chain, error) {
	if genesisConfig, parseErr := chain.Import(
		genesisPath,
	); parseErr != nil {
		return nil, parseErr
	} else {
		return genesisConfig, nil
	}
}

func GetCommand() *cobra.Command {
	reverifyCmd := &cobra.Command{
		Use:     "reverify",
		Short:   "Reverify block data",
		PreRunE: runPreRun,
		RunE:    runCommand,
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

func runCommand(cmd *cobra.Command, _ []string) error {
	command.InitializePprofServer(cmd)

	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "reverify",
		Level: hclog.Info,
	})

	verifyStartHeight := params.startHeight
	if verifyStartHeight <= 0 {
		return fmt.Errorf("verify height must be greater than 0")
	}

	chain, err := parseGenesis(params.GenesisPath)
	if err != nil {
		logger.Error("failed to parse genesis")

		return err
	}

	return reverify.ReverifyChain(
		logger,
		chain,
		params.DataDir,
		verifyStartHeight,
	)
}
