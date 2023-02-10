package status

import (
	"context"
	"time"

	"github.com/dogechain-lab/dogechain/command"
	"github.com/dogechain-lab/dogechain/command/helper"
	"github.com/spf13/cobra"

	txpoolOp "github.com/dogechain-lab/dogechain/txpool/proto"
	empty "google.golang.org/protobuf/types/known/emptypb"
)

func GetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Returns the number of transactions in the transaction pool",
		Run:   runCommand,
	}
}

func runCommand(cmd *cobra.Command, _ []string) {
	outputter := command.InitializeOutputter(cmd)
	defer outputter.WriteOutput()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	statusResponse, err := getTxPoolStatus(ctx, helper.GetGRPCAddress(cmd))
	if err != nil {
		outputter.SetError(err)

		return
	}

	outputter.SetCommandResult(&TxPoolStatusResult{
		PendingTransactions:  statusResponse.PendingLength,
		EnqueuedTransactions: statusResponse.EnqueuedLength,
		MaxSlots:             statusResponse.MaxSlots,
		CurrentSlots:         statusResponse.CurrentSlots,
	})
}

func getTxPoolStatus(ctx context.Context, grpcAddress string) (*txpoolOp.TxnPoolStatusResp, error) {
	client, err := helper.GetTxPoolClientConnection(
		ctx,
		grpcAddress,
	)
	if err != nil {
		return nil, err
	}

	return client.Status(context.Background(), &empty.Empty{})
}
