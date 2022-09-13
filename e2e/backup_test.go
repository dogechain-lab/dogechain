package e2e

import (
	"context"
	"os"
	"path"
	"testing"
	"time"

	"github.com/dogechain-lab/dogechain/archive"
	"github.com/dogechain-lab/dogechain/command/helper"
	"github.com/dogechain-lab/dogechain/e2e/framework"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/umbracle/go-web3"
)

func TestBackup(t *testing.T) {
	compressionFile := path.Join(os.TempDir(), "e2e_testbackup.bin.zstd")
	noCompressionFile := path.Join(os.TempDir(), "e2e_testbackup.bin")
	backupFiles := []string{noCompressionFile, compressionFile}

	var toBlock uint64 = 10

	svrs := framework.NewTestServers(t, 4, func(config *framework.TestServerConfig) {
		config.SetConsensus(framework.ConsensusDev)
		config.SetSeal(true)
		config.SetDevInterval(1)
	})

	svr := svrs[0]

	errs := framework.WaitForServersToSeal(svrs, toBlock)
	for _, err := range errs {
		assert.NoError(t, err)
	}

	connection, err := helper.GetGRPCConnection(
		svr.GrpcAddr(),
	)

	assert.NoError(t, err)

	for _, backupFile := range backupFiles {
		resFrom, resTo, err := archive.CreateBackup(
			connection,
			hclog.NewNullLogger(),
			0,
			&toBlock,
			backupFile,
			true,
			false,
			3,
		)

		assert.NoError(t, err)
		assert.Equal(t, uint64(0), resFrom)
		assert.Equal(t, uint64(10), resTo)

		t.Cleanup(func() {
			os.Remove(backupFile)
		})
	}

	block, err := svr.JSONRPC().Eth().GetBlockByNumber(web3.BlockNumber(toBlock), false)
	assert.NoError(t, err)

	blockHash := block.Hash

	for _, svr := range svrs {
		svr.Stop()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, backupFile := range backupFiles {
		os.RemoveAll(path.Join(svr.Config.RootDir, "blockchain"))
		os.RemoveAll(path.Join(svr.Config.RootDir, "trie"))

		restoreSvr := framework.NewTestServer(t, svr.Config.RootDir, func(config *framework.TestServerConfig) {
			*config = *svr.Config
			config.SetRestoreFile(backupFile)
		})

		err := restoreSvr.Start(ctx)
		assert.NoError(t, err)

		block, err := restoreSvr.JSONRPC().Eth().GetBlockByNumber(web3.BlockNumber(toBlock), false)
		assert.NoError(t, err)

		restoreHash := block.Hash

		assert.Equal(t, blockHash, restoreHash)

		restoreSvr.Stop()
	}
}
