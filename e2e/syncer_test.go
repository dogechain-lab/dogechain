package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/dogechain-lab/dogechain/e2e/framework"
	"github.com/stretchr/testify/assert"
	"github.com/umbracle/go-web3"
)

func TestClusterBlockSync(t *testing.T) {
	const (
		numNonValidators = 2
		desiredHeight    = 10
	)

	// Start IBFT cluster (4 Validator + 2 Non-Validator)
	ibftManager := framework.NewIBFTServersManager(
		t,
		IBFTMinNodes+numNonValidators,
		IBFTDirPrefix,
		false,
		func(i int, config *framework.TestServerConfig) {
			if i >= IBFTMinNodes {
				// Other nodes should not be in the validator set
				dirPrefix := "dogechain-non-validator-"
				config.SetIBFTDirPrefix(dirPrefix)
				config.SetIBFTDir(fmt.Sprintf("%s%d", dirPrefix, i))
			}
			config.SetSeal(i < IBFTMinNodes)
			config.SetBlockTime(1)
		})

	startContext, startCancelFn := context.WithTimeout(context.Background(), time.Minute)
	defer startCancelFn()
	ibftManager.StartServers(startContext)

	servers := make([]*framework.TestServer, 0)
	for i := 0; i < IBFTMinNodes+numNonValidators; i++ {
		servers = append(servers, ibftManager.GetServer(i))
	}
	// All nodes should have mined the same block eventually
	waitErrors := framework.WaitForServersToSeal(servers, desiredHeight)

	if len(waitErrors) != 0 {
		t.Fatalf("Unable to wait for all nodes to seal blocks, %v", waitErrors)
	}

	// should get the same block results no matter which one to query
	var (
		blocks = make([]*web3.Block, len(servers))
		err    error
	)

	// take a long time, but we must take it
	for idx, server := range servers {
		blocks[idx], err = server.JSONRPC().Eth().GetBlockByNumber(desiredHeight, false)
		assert.NoError(t, err)
	}

	expectedBlock := blocks[0]

	assert.NotNil(t, expectedBlock)

	for _, block := range blocks {
		assert.Equal(t, expectedBlock, block)
	}
}
