package add

import (
	"context"
	"errors"

	"github.com/dogechain-lab/dogechain/command"
	"github.com/dogechain-lab/dogechain/command/helper"
	"github.com/dogechain-lab/dogechain/server/proto"
)

var (
	params = &addParams{
		addedPeers: make([]string, 0),
		addErrors:  make([]string, 0),
	}
)

var (
	errInvalidAddresses = errors.New("at least 1 peer address is required")
)

const (
	addrFlag   = "addr"
	staticFlag = "static"
)

type addParams struct {
	isStatic      bool
	peerAddresses []string

	systemClient proto.SystemClient

	addedPeers []string
	addErrors  []string
}

func (p *addParams) getRequiredFlags() []string {
	return []string{
		addrFlag,
	}
}

func (p *addParams) validateFlags() error {
	if len(p.peerAddresses) < 1 {
		return errInvalidAddresses
	}

	return nil
}

func (p *addParams) initSystemClient(grpcAddress string) error {
	systemClient, err := helper.GetSystemClientConnection(grpcAddress)
	if err != nil {
		return err
	}

	p.systemClient = systemClient

	return nil
}

func (p *addParams) addPeers() {
	for _, address := range p.peerAddresses {
		if addErr := p.addPeer(address, p.isStatic); addErr != nil {
			p.addErrors = append(p.addErrors, addErr.Error())

			continue
		}

		p.addedPeers = append(p.addedPeers, address)
	}
}

func (p *addParams) addPeer(peerAddress string, static bool) error {
	if _, err := p.systemClient.PeersAdd(
		context.Background(),
		&proto.PeersAddRequest{
			Id:     peerAddress,
			Static: static,
		},
	); err != nil {
		return err
	}

	return nil
}

func (p *addParams) getResult() command.CommandResult {
	return &PeersAddResult{
		NumRequested: len(p.peerAddresses),
		NumAdded:     len(p.addedPeers),
		Peers:        p.addedPeers,
		Errors:       p.addErrors,
	}
}
