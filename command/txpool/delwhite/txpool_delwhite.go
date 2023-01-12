package delwhite

import (
	"github.com/dogechain-lab/dogechain/command"
	"github.com/dogechain-lab/dogechain/command/helper"
	"github.com/spf13/cobra"
)

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delwhite",
		Short:   "Delete contract from ddos whitelist",
		PreRunE: runPreRunE,
		Run:     runCommand,
	}

	setFlags(cmd)
	helper.SetRequiredFlags(cmd, params.getRequiredFlags())

	return cmd
}

func setFlags(cmd *cobra.Command) {
	cmd.Flags().StringArrayVar(
		&params.contractAddresses,
		addrFlag,
		[]string{},
		"the contract addresses need add to ddos whitelist",
	)
}

func runPreRunE(_ *cobra.Command, _ []string) error {
	return params.validateFlags()
}

func runCommand(cmd *cobra.Command, _ []string) {
	outputter := command.InitializeOutputter(cmd)
	defer outputter.WriteOutput()

	if err := params.initSystemClient(helper.GetGRPCAddress(cmd)); err != nil {
		outputter.SetError(err)

		return
	}

	params.deleteWhitelistContracts()

	outputter.SetCommandResult(params.getResult())
}
