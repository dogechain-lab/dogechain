package verify

import (
	"github.com/dogechain-lab/dogechain/types"
)

const (
	dataDirFlag = "data-dir"
	genesisPath = "chain"
	startHeight = "start-height"
)

var (
	params = &verifyParams{}
)

type verifyParams struct {
	DataDir     string
	GenesisPath string

	startHeightRaw string
	startHeight    uint64
}

func (p *verifyParams) validateFlags() error {
	var parseErr error

	if p.startHeight, parseErr = types.ParseUint64orHex(&p.startHeightRaw); parseErr != nil {
		return parseErr
	}

	return nil
}

func (p *verifyParams) getRequiredFlags() []string {
	return []string{
		dataDirFlag,
		startHeight,
	}
}
