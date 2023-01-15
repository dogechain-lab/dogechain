package delwhite

import (
	"context"
	"errors"
	"time"

	"github.com/dogechain-lab/dogechain/command"
	"github.com/dogechain-lab/dogechain/command/helper"
	"github.com/dogechain-lab/dogechain/server/proto"
)

var (
	params = &inoutParam{
		contractAddresses: make([]string, 0),
	}
)

var (
	errInvalidAddresses = errors.New("at least 1 contract address is required")
)

const (
	addrFlag = "addr"
)

type inoutParam struct {
	systemClient      proto.SystemClient
	contractAddresses []string
	deletedNum        int64
	err               error
}

func (p *inoutParam) getRequiredFlags() []string {
	return []string{
		addrFlag,
	}
}

func (p *inoutParam) validateFlags() error {
	if len(p.contractAddresses) < 1 {
		return errInvalidAddresses
	}

	return nil
}

func (p *inoutParam) initSystemClient(grpcAddress string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	systemClient, err := helper.GetSystemClientConnection(ctx, grpcAddress)
	if err != nil {
		return err
	}

	p.systemClient = systemClient

	return nil
}

func (p *inoutParam) deleteWhitelistContracts() {
	rsp, err := p.systemClient.WhitelistDeleteList(
		context.Background(),
		&proto.WhitelistDeleteListRequest{
			Contracts: p.contractAddresses,
		},
	)
	if err != nil {
		p.err = err

		return
	}

	p.deletedNum = rsp.Count
}

func (p *inoutParam) getResult() command.CommandResult {
	return &Result{
		Contracts:  p.contractAddresses,
		NumDeleted: p.deletedNum,
		Error:      p.err,
	}
}
