package e2e

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dogechain-lab/dogechain/chain"
	"github.com/dogechain-lab/dogechain/e2e/framework"
	"github.com/dogechain-lab/dogechain/helper/kvdb"
	"github.com/dogechain-lab/dogechain/reverify"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

func TestReverify(t *testing.T) {
	const toBlock uint64 = 10

	const finalToBlock uint64 = 15

	svrs := framework.NewTestServers(t, 4, func(config *framework.TestServerConfig) {
		config.SetConsensus(framework.ConsensusDev)
		config.SetSeal(true)
		config.SetDevInterval(1)
	})

	{
		errs := framework.WaitForServersToSeal(svrs, toBlock)
		for _, err := range errs {
			assert.NoError(t, err)
		}
	}

	svr := svrs[0]
	svrRootDir := svr.Config.RootDir
	svr.Stop()

	time.Sleep(time.Second * 2)

	// open trie database
	leveldbBuilder := kvdb.NewLevelDBBuilder(
		hclog.NewNullLogger(),
		filepath.Join(svrRootDir, "trie"),
	)

	// open chain database
	trie, err := leveldbBuilder.Build()
	assert.NoError(t, err)

	// corrupt data
	{
		iter := trie.Iterator(nil)
		assert.NoError(t, iter.Error())

		iter.Last()
		assert.NoError(t, iter.Error())

		for iter.Prev() {
			assert.NoError(t, iter.Error())
			trie.Set(iter.Key(), []byte("corrupted data"))
		}
		iter.Release()
		err := trie.Close()

		assert.NoError(t, err)
	}

	genesis, parseErr := chain.Import(
		filepath.Join(svrRootDir, "genesis.json"),
	)
	assert.NoError(t, parseErr)

	err = reverify.ReverifyChain(
		hclog.NewNullLogger(),
		genesis,
		svrRootDir,
		1,
	)

	assert.NoError(t, err)

	resvr := framework.NewTestServer(t, svrRootDir, func(config *framework.TestServerConfig) {
		*config = *svr.Config
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)

	err = resvr.Start(ctx)
	assert.NoError(t, err)
	t.Cleanup(func() {
		resvr.Stop()
	})

	_, err = framework.WaitUntilBlockMined(ctx, resvr, finalToBlock)
	assert.NoError(t, err)
}
