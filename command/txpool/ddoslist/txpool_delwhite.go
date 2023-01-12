package ddoslist

import (
	"github.com/dogechain-lab/dogechain/command"
	"github.com/dogechain-lab/dogechain/command/helper"
	"github.com/spf13/cobra"
)

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ddoslist",
		Short: "Query ddos contract list",
		Run:   runCommand,
	}

	return cmd
}

func runCommand(cmd *cobra.Command, _ []string) {
	outputter := command.InitializeOutputter(cmd)
	defer outputter.WriteOutput()

	if err := params.initSystemClient(helper.GetGRPCAddress(cmd)); err != nil {
		outputter.SetError(err)

		return
	}

	params.queryDDOSList()

	outputter.SetCommandResult(params.getResult())
}
