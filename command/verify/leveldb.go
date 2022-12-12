package verify

import (
	"github.com/dogechain-lab/dogechain/helper/kvdb"
	"github.com/hashicorp/go-hclog"
)

func newLevelDBBuilder(log hclog.Logger, path string) kvdb.LevelDBBuilder {
	leveldbBuilder := kvdb.NewLevelDBBuilder(
		log,
		path,
	)
	leveldbBuilder.SetNoSync(true)

	return leveldbBuilder
}
