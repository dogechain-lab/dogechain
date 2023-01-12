package txpool

import (
	"github.com/dogechain-lab/dogechain/command/helper"
	"github.com/dogechain-lab/dogechain/command/txpool/addwhite"
	"github.com/dogechain-lab/dogechain/command/txpool/ddoslist"
	"github.com/dogechain-lab/dogechain/command/txpool/delwhite"
	"github.com/dogechain-lab/dogechain/command/txpool/status"
	"github.com/dogechain-lab/dogechain/command/txpool/subscribe"
	"github.com/spf13/cobra"
)

func GetCommand() *cobra.Command {
	txPoolCmd := &cobra.Command{
		Use:   "txpool",
		Short: "Top level command for interacting with the transaction pool. Only accepts subcommands.",
	}

	helper.RegisterGRPCAddressFlag(txPoolCmd)

	registerSubcommands(txPoolCmd)

	return txPoolCmd
}

func registerSubcommands(baseCmd *cobra.Command) {
	baseCmd.AddCommand(
		// txpool status
		status.GetCommand(),
		// txpool subscribe
		subscribe.GetCommand(),
		// txpool add ddos whitelist
		addwhite.GetCommand(),
		// txpool delete ddos whitelist
		delwhite.GetCommand(),
		// txpool ddos list
		ddoslist.GetCommand(),
	)
}
