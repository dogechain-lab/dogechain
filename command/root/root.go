package root

import (
	"fmt"
	"os"

	"github.com/dogechain-lab/dogechain/command/backup"
	"github.com/dogechain-lab/dogechain/command/genesis"
	"github.com/dogechain-lab/dogechain/command/helper"
	"github.com/dogechain-lab/dogechain/command/ibft"
	"github.com/dogechain-lab/dogechain/command/license"
	"github.com/dogechain-lab/dogechain/command/loadbot"
	"github.com/dogechain-lab/dogechain/command/monitor"
	"github.com/dogechain-lab/dogechain/command/peers"
	"github.com/dogechain-lab/dogechain/command/secrets"
	"github.com/dogechain-lab/dogechain/command/server"
	"github.com/dogechain-lab/dogechain/command/status"
	"github.com/dogechain-lab/dogechain/command/txpool"
	"github.com/dogechain-lab/dogechain/command/version"
	"github.com/spf13/cobra"
)

type RootCommand struct {
	baseCmd *cobra.Command
}

func NewRootCommand() *RootCommand {
	rootCommand := &RootCommand{
		baseCmd: &cobra.Command{
			Short: "Dogechain-Lab Dogechain is a framework for building Ethereum-compatible Blockchain networks",
		},
	}

	helper.RegisterJSONOutputFlag(rootCommand.baseCmd)

	rootCommand.registerSubCommands()

	return rootCommand
}

func (rc *RootCommand) registerSubCommands() {
	rc.baseCmd.AddCommand(
		version.GetCommand(),
		txpool.GetCommand(),
		status.GetCommand(),
		secrets.GetCommand(),
		peers.GetCommand(),
		monitor.GetCommand(),
		loadbot.GetCommand(),
		ibft.GetCommand(),
		backup.GetCommand(),
		genesis.GetCommand(),
		server.GetCommand(),
		license.GetCommand(),
	)
}

func (rc *RootCommand) Execute() {
	if err := rc.baseCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)

		os.Exit(1)
	}
}
