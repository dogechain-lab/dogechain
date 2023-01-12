package ddoslist

import (
	"context"

	"github.com/dogechain-lab/dogechain/command"
	"github.com/dogechain-lab/dogechain/command/helper"
	"github.com/dogechain-lab/dogechain/server/proto"
	"github.com/golang/protobuf/ptypes/empty"
)

var (
	params = &inoutParam{}
)

type inoutParam struct {
	systemClient proto.SystemClient
	blacklist    map[string]int64
	whitelist    map[string]int64
	err          error
}

func (p *inoutParam) initSystemClient(grpcAddress string) error {
	systemClient, err := helper.GetSystemClientConnection(grpcAddress)
	if err != nil {
		return err
	}

	p.systemClient = systemClient

	return nil
}

func (p *inoutParam) queryDDOSList() {
	rsp, err := p.systemClient.DDOSContractList(
		context.Background(),
		&empty.Empty{},
	)
	if err != nil {
		p.err = err

		return
	}

	p.blacklist = rsp.Blacklist
	p.whitelist = rsp.Whitelist
}

func (p *inoutParam) getResult() command.CommandResult {
	return &Result{
		Blacklist: p.blacklist,
		Whitelist: p.whitelist,
		Error:     p.err,
	}
}
