package reverify

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/dogechain-lab/dogechain/blockchain"
	"github.com/dogechain-lab/dogechain/blockchain/storage/kvstorage"
	"github.com/dogechain-lab/dogechain/chain"
	"github.com/dogechain-lab/dogechain/consensus"
	"github.com/dogechain-lab/dogechain/helper/kvdb"
	"github.com/dogechain-lab/dogechain/network"
	"github.com/dogechain-lab/dogechain/secrets"
	"github.com/dogechain-lab/dogechain/server"
	"github.com/dogechain-lab/dogechain/state"
	"github.com/hashicorp/go-hclog"

	itrie "github.com/dogechain-lab/dogechain/state/immutable-trie"
	"github.com/dogechain-lab/dogechain/state/runtime/evm"
	"github.com/dogechain-lab/dogechain/state/runtime/precompiled"
)

func newLevelDBBuilder(log hclog.Logger, path string) kvdb.LevelDBBuilder {
	leveldbBuilder := kvdb.NewLevelDBBuilder(
		log,
		path,
	)

	return leveldbBuilder
}

func createConsensus(
	logger hclog.Logger,
	genesis *chain.Chain,
	blockchain *blockchain.Blockchain,
	executor *state.Executor,
	dataDir string,
) (consensus.Consensus, error) {
	engineName := genesis.Params.GetEngine()
	engine, ok := server.GetConsensusBackend(engineName)

	if !ok {
		return nil, fmt.Errorf("consensus engine '%s' not found", engineName)
	}

	secretsManagerFactory, ok := server.GetSecretsManager(secrets.Local)
	if !ok {
		return nil, fmt.Errorf("secret manager '%s' not found", secrets.Local)
	}

	// Instantiate the secrets manager
	secretsManager, factoryErr := secretsManagerFactory(
		&secrets.SecretsManagerConfig{},
		&secrets.SecretsManagerParams{
			Logger: logger,
			Extra: map[string]interface{}{
				secrets.Path: dataDir,
			},
		},
	)

	if factoryErr != nil {
		return nil, factoryErr
	}

	engineConfig, ok := genesis.Params.Engine[engineName].(map[string]interface{})
	if !ok {
		engineConfig = map[string]interface{}{}
	}

	config := &consensus.Config{
		Params: genesis.Params,
		Config: engineConfig,
		Path:   filepath.Join(dataDir, "consensus"),
	}

	consensus, err := engine(
		&consensus.ConsensusParams{
			Context:        context.Background(),
			Seal:           false,
			Config:         config,
			Txpool:         nil,
			Network:        &network.NonetworkServer{},
			Blockchain:     blockchain,
			Executor:       executor,
			Grpc:           nil,
			Logger:         logger.Named("consensus"),
			Metrics:        nil,
			SecretsManager: secretsManager,
			BlockTime:      2,
			BlockBroadcast: false,
		},
	)

	if err != nil {
		return nil, err
	}

	return consensus, nil
}

func createBlockchain(
	logger hclog.Logger,
	genesis *chain.Chain,
	st *itrie.State,
	dataDir string,
) (*blockchain.Blockchain, consensus.Consensus, error) {
	executor := state.NewExecutor(genesis.Params, st, logger)
	executor.SetRuntime(precompiled.NewPrecompiled())
	executor.SetRuntime(evm.NewEVM())

	genesisRoot := executor.WriteGenesis(genesis.Genesis.Alloc)
	genesis.Genesis.StateRoot = genesisRoot

	chain, err := blockchain.NewBlockchain(
		logger,
		genesis,
		kvstorage.NewLevelDBStorageBuilder(
			logger,
			newLevelDBBuilder(logger, filepath.Join(dataDir, "blockchain")),
		),
		nil,
		executor,
		nil,
	)
	if err != nil {
		return nil, nil, err
	}

	executor.GetHash = chain.GetHashHelper

	consensus, err := createConsensus(logger, genesis, chain, executor, dataDir)
	if err != nil {
		return nil, nil, err
	}

	chain.SetConsensus(consensus)

	if err := chain.ComputeGenesis(); err != nil {
		return nil, nil, err
	}

	// initialize data in consensus layer
	if err := consensus.Initialize(); err != nil {
		return nil, nil, err
	}

	if err := consensus.Start(); err != nil {
		return nil, nil, err
	}

	return chain, consensus, nil
}
