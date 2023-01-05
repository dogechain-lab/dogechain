package jsonrpc

import (
	"fmt"

	"github.com/dogechain-lab/dogechain/helper/hex"
	"github.com/dogechain-lab/dogechain/helper/keccak"
	"github.com/dogechain-lab/dogechain/versioning"
)

// Web3 is the web3 jsonrpc endpoint
type Web3 struct {
	chainID uint64

	metrics *Metrics
}

var _clientVersionTemplate = "dogechain [chain-id: %d] [version: %s]"

// ClientVersion returns the version of the web3 client (web3_clientVersion)
func (w *Web3) ClientVersion() (interface{}, error) {
	w.metrics.Web3APICounterInc(Web3ClientVersionLabel)

	return fmt.Sprintf(
		_clientVersionTemplate,
		w.chainID,
		versioning.Version,
	), nil
}

// Sha3 returns Keccak-256 (not the standardized SHA3-256) of the given data
func (w *Web3) Sha3(val string) (interface{}, error) {
	w.metrics.Web3APICounterInc(Web3Sha3Label)

	v, err := hex.DecodeHex(val)
	if err != nil {
		return nil, NewInvalidRequestError("Invalid hex string")
	}

	dst := keccak.Keccak256(nil, v)

	return hex.EncodeToHex(dst), nil
}
