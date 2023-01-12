package addwhite

import (
	"context"
	"errors"

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
	addedNum          int64
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
	systemClient, err := helper.GetSystemClientConnection(grpcAddress)
	if err != nil {
		return err
	}

	p.systemClient = systemClient

	return nil
}

func (p *inoutParam) addWhitelistContracts() {
	rsp, err := p.systemClient.WhitelistAddList(
		context.Background(),
		&proto.WhitelistAddListRequest{
			Contracts: p.contractAddresses,
		},
	)
	if err != nil {
		p.err = err

		return
	}

	p.addedNum = rsp.Count
}

func (p *inoutParam) getResult() command.CommandResult {
	return &Result{
		Contracts: p.contractAddresses,
		NumAdded:  p.addedNum,
		Error:     p.err,
	}
}
