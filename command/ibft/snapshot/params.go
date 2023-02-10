package snapshot

import (
	"context"
	"time"

	"github.com/dogechain-lab/dogechain/command"
	"github.com/dogechain-lab/dogechain/command/helper"
	ibftOp "github.com/dogechain-lab/dogechain/consensus/ibft/proto"
)

const (
	numberFlag = "number"
)

var (
	params = &snapshotParams{}
)

type snapshotParams struct {
	blockNumber int

	snapshot *ibftOp.Snapshot
}

func (p *snapshotParams) initSnapshot(grpcAddress string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	ibftClient, err := helper.GetIBFTOperatorClientConnection(ctx, grpcAddress)
	if err != nil {
		return err
	}

	snapshot, err := ibftClient.GetSnapshot(
		context.Background(),
		p.getSnapshotRequest(),
	)
	if err != nil {
		return err
	}

	p.snapshot = snapshot

	return nil
}

func (p *snapshotParams) getSnapshotRequest() *ibftOp.SnapshotReq {
	req := &ibftOp.SnapshotReq{
		Latest: true,
	}

	if p.blockNumber >= 0 {
		req.Latest = false
		req.Number = uint64(p.blockNumber)
	}

	return req
}

func (p *snapshotParams) getResult() command.CommandResult {
	return newIBFTSnapshotResult(p.snapshot)
}
