package server

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/dogechain-lab/dogechain/archive"
	"github.com/dogechain-lab/dogechain/blockchain"
	"github.com/dogechain-lab/dogechain/blockchain/storage/kvstorage"
	"github.com/dogechain-lab/dogechain/chain"
	"github.com/dogechain-lab/dogechain/consensus"
	"github.com/dogechain-lab/dogechain/crypto"
	"github.com/dogechain-lab/dogechain/graphql"
	"github.com/dogechain-lab/dogechain/helper/common"
	"github.com/dogechain-lab/dogechain/helper/kvdb"
	"github.com/dogechain-lab/dogechain/helper/kvdb/leveldb"
	"github.com/dogechain-lab/dogechain/helper/progress"
	"github.com/dogechain-lab/dogechain/helper/rawdb"
	"github.com/dogechain-lab/dogechain/jsonrpc"
	"github.com/dogechain-lab/dogechain/network"
	"github.com/dogechain-lab/dogechain/secrets"
	"github.com/dogechain-lab/dogechain/server/proto"
	"github.com/dogechain-lab/dogechain/state"
	itrie "github.com/dogechain-lab/dogechain/state/immutable-trie"
	"github.com/dogechain-lab/dogechain/state/runtime/evm"
	"github.com/dogechain-lab/dogechain/state/runtime/precompiled"
	"github.com/dogechain-lab/dogechain/state/snapshot"
	"github.com/dogechain-lab/dogechain/state/stypes"
	"github.com/dogechain-lab/dogechain/state/utils"
	"github.com/dogechain-lab/dogechain/trie"
	"github.com/dogechain-lab/dogechain/txpool"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/hashicorp/go-hclog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
)

// Minimal is the central manager of the blockchain client
type Server struct {
	// configuration
	config *Config

	// global logger
	logger hclog.Logger

	// databases
	trieDB    itrie.Storage
	chainDB   kvdb.Database
	snpTrieDB *trie.Database // snapshots usage trie database

	// consensus
	consensus consensus.Consensus

	// blockchain stack
	blockchain *blockchain.Blockchain

	// state holder
	state state.State
	// state executor
	executor *state.Executor
	// state snapshots
	snaps *snapshot.Tree
	// Cache configuration for pruning
	cacheConfig *CacheConfig

	// jsonrpc stack
	jsonrpcServer *jsonrpc.JSONRPC
	// graphql stack
	graphqlServer *graphql.GraphQLService

	// system grpc server
	grpcServer *grpc.Server

	// libp2p network
	network network.Server

	// transaction pool
	txpool *txpool.TxPool

	serverMetrics *serverMetrics

	prometheusServer *http.Server

	// secrets manager
	secretsManager secrets.SecretsManager

	// restore
	restoreProgression *progress.ProgressionWrapper
}

const (
	loggerDomainName  = "dogechain"
	BlockchainDataDir = "blockchain"
	StateDataDir      = "trie"
	TrieCacheDir      = "triecache"
)

var dirPaths = []string{
	BlockchainDataDir,
	StateDataDir,
	TrieCacheDir,
}

// newFileLogger returns logger instance that writes all logs to a specified file.
//
// If log file can't be created, it returns an error
func newFileLogger(config *Config) (hclog.Logger, error) {
	logFileWriter, err := os.OpenFile(
		config.LogFilePath,
		os.O_CREATE+os.O_RDWR+os.O_APPEND,
		0640,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create log file, %w", err)
	}

	return hclog.New(&hclog.LoggerOptions{
		Name:   loggerDomainName,
		Level:  config.LogLevel,
		Output: logFileWriter,
	}), nil
}

// newCLILogger returns minimal logger instance that sends all logs to standard output
func newCLILogger(config *Config) hclog.Logger {
	return hclog.New(&hclog.LoggerOptions{
		Name:  loggerDomainName,
		Level: config.LogLevel,
	})
}

// newLoggerFromConfig creates a new logger which logs to a specified file.
//
// If log file is not set it outputs to standard output ( console ).
// If log file is specified, and it can't be created the server command will error out
func newLoggerFromConfig(config *Config) (hclog.Logger, error) {
	if config.LogFilePath != "" {
		fileLoggerInstance, err := newFileLogger(config)
		if err != nil {
			return nil, err
		}

		return fileLoggerInstance, nil
	}

	return newCLILogger(config), nil
}

// NewServer creates a new Minimal server, using the passed in configuration
func NewServer(config *Config) (*Server, error) {
	logger, err := newLoggerFromConfig(config)
	if err != nil {
		return nil, fmt.Errorf("could not setup new logger instance, %w", err)
	}

	hclog.SetDefault(logger)

	srv := &Server{
		logger:      logger,
		config:      config,
		cacheConfig: config.CacheConfig,
		grpcServer: grpc.NewServer(
			grpc.MaxRecvMsgSize(common.MaxGrpcMsgSize),
			grpc.MaxSendMsgSize(common.MaxGrpcMsgSize),
		),
		restoreProgression: progress.NewProgressionWrapper(progress.ChainSyncRestore),
	}

	srv.logger.Info("Data dir", "path", config.DataDir)

	// Generate all the paths in the dataDir
	if err := common.SetupDataDir(config.DataDir, dirPaths); err != nil {
		return nil, fmt.Errorf("failed to create data directories: %w", err)
	}

	// Set up metrics
	srv.setupMetris()

	// Set up the secrets manager
	if err := srv.setupSecretsManager(); err != nil {
		return nil, fmt.Errorf("failed to set up the secrets manager: %w", err)
	}

	// start libp2p
	if err := srv.setupNetwork(); err != nil {
		return nil, err
	}

	// set up blockchain database
	if err := srv.setupBlockchainDB(); err != nil {
		return nil, err
	}

	// set up state database
	if err := srv.setupStateDB(); err != nil {
		return nil, err
	}

	// set up trie database
	srv.setupSnpTrieDB(srv.trieDB)

	// setup executor
	if err := srv.setupExecutor(); err != nil {
		return nil, err
	}

	// setup world state snapshots
	if err := srv.setupSnapshots(); err != nil {
		return nil, err
	}

	// setup blockchain
	if err := srv.setupBlockchain(); err != nil {
		return nil, err
	}

	// Set up txpool
	if err := srv.setupTxpool(); err != nil {
		return nil, err
	}

	// Setup consensus
	if err := srv.setupConsensus(); err != nil {
		return nil, err
	}

	// after consensus is done, we can mine the genesis block in blockchain
	// This is done because consensus might use a custom Hash function so we need
	// to wait for consensus because we do any block hashing like genesis
	if err := srv.blockchain.ComputeGenesis(); err != nil {
		return nil, err
	}

	// initialize data in consensus layer
	if err := srv.consensus.Initialize(); err != nil {
		return nil, err
	}

	// setup and start jsonrpc server
	if err := srv.setupJSONRPC(); err != nil {
		return nil, err
	}

	// setup and start graphql server
	if err := srv.setupGraphQL(); err != nil {
		return nil, err
	}

	// restore archive data before starting
	if err := srv.restoreChain(); err != nil {
		return nil, err
	}

	// start consensus
	if err := srv.consensus.Start(); err != nil {
		return nil, err
	}

	// setup and start grpc server
	if err := srv.setupGRPC(); err != nil {
		return nil, err
	}

	// start network to discover peers
	if err := srv.network.Start(); err != nil {
		return nil, err
	}

	// start txpool to accept transactions
	srv.txpool.Start()

	return srv, nil
}

// setupTxpool set up txpool
//
// Must behind other components initilized
func (s *Server) setupTxpool() error {
	var (
		err    error
		config = s.config
		hub    = &txpoolHub{
			snaps:      s.snaps,
			state:      s.state,
			Blockchain: s.blockchain,
		}
		chainCfg = s.config.Chain
	)

	blackList := make([]types.Address, len(chainCfg.Params.BlackList))
	for i, a := range chainCfg.Params.BlackList {
		blackList[i] = types.StringToAddress(a)
	}

	txpoolCfg := &txpool.Config{
		Sealing:               config.Seal,
		MaxSlots:              config.MaxSlots,
		PriceLimit:            config.PriceLimit,
		PruneTickSeconds:      config.PruneTickSeconds,
		PromoteOutdateSeconds: config.PromoteOutdateSeconds,
		BlackList:             blackList,
		DDOSProtection:        chainCfg.Params.DDOSProtection,
	}

	// start transaction pool
	s.txpool, err = txpool.NewTxPool(
		s.logger,
		chainCfg.Params.Forks.At(0),
		hub,
		s.grpcServer,
		s.network,
		s.serverMetrics.txpool,
		txpoolCfg,
	)
	if err != nil {
		return err
	}

	// use the eip155 signer at the beginning
	signer := crypto.NewEIP155Signer(uint64(chainCfg.Params.ChainID))
	s.txpool.SetSigner(signer)

	return nil
}

func (s *Server) GetHashHelper(header *types.Header) func(uint64) types.Hash {
	return func(u uint64) types.Hash {
		v, _ := rawdb.ReadCanonicalHash(s.chainDB, u)

		return v
	}
}

func (s *Server) setupBlockchain() error {
	bc, err := blockchain.NewBlockchain(
		s.logger,
		s.config.Chain,
		nil,
		kvstorage.NewKeyValueStorage(s.chainDB),
		s.executor,
		s.serverMetrics.blockchain,
	)
	if err != nil {
		return err
	}

	// blockchain object
	s.blockchain = bc

	return nil
}

// Load any existing snapshot, regenerating it if loading failed
func (s *Server) setupSnapshots() error {
	if s.cacheConfig.SnapshotLimit <= 0 {
		return nil
	}

	var (
		needRecover   bool
		logger        = s.logger.Named("snapshots")
		chainDB       = s.chainDB
		trieDB        = s.trieDB
		headStateRoot types.Hash
		headNumber    uint64
		isGenesis     bool
	)

	headHash, exists := rawdb.ReadHeadHash(chainDB)
	if !exists {
		s.logger.Warn("head hash not found, might generate from genesis")

		isGenesis = true

		// get genesis root
		headStateRoot = s.config.Chain.Genesis.StateRoot
		if headStateRoot == types.ZeroHash {
			return nil
		}
	}

	if !isGenesis {
		header, err := rawdb.ReadHeader(chainDB, headHash)
		if err != nil {
			s.logger.Error("get header failed", "err", err)
			os.Exit(1)
		}

		headNumber = header.Number
		headStateRoot = header.StateRoot
	}

	// If the chain was rewound past the snapshot persistent layer (causing a recovery
	// block number to be persisted to disk), check if we're still in recovery mode
	// and in that case, don't invalidate the snapshot on a head mismatch.
	// NOTE: It is impossible in pos chain, the only one exception is hacked database
	if layer := rawdb.ReadSnapshotRecoveryNumber(trieDB); layer != nil && *layer >= headNumber {
		s.logger.Warn("Enabling snapshot recovery", "chainhead", headNumber, "diskbase", *layer)

		needRecover = true
	}

	snapCfg := snapshot.Config{
		CacheSize: s.cacheConfig.SnapshotLimit,
		Recovery:  needRecover,
		// background snapshots, if we use it in command line, it should be disabled
		NoBuild:    !s.config.EnableSnapshot,
		AsyncBuild: !s.cacheConfig.SnapshotWait,
	}

	snaps, err := snapshot.New(snapCfg, trieDB, s.snpTrieDB, headStateRoot, logger, s.serverMetrics.snapshot)
	if err != nil {
		return err
	}

	// server snaps for rebuild and dump
	s.snaps = snaps

	// update executor's snaps
	if s.executor != nil {
		s.executor.SetSnaps(snaps)
	}

	return nil
}

func (s *Server) setupExecutor() error {
	logger := s.logger.Named("executor")
	executor := state.NewExecutor(
		s.config.Chain.Params,
		logger,
		s.state,
	)
	// other properties
	executor.SetRuntime(precompiled.NewPrecompiled())
	executor.SetRuntime(evm.NewEVM())
	executor.GetHash = s.GetHashHelper

	// compute the genesis root state
	// TODO: weird to commit every restart
	genesisRoot, err := executor.WriteGenesis(s.config.Chain.Genesis.Alloc)
	if err != nil {
		return err
	}

	// set executor
	s.executor = executor

	// update genesis state root
	s.config.Chain.Genesis.StateRoot = genesisRoot

	return nil
}

func (s *Server) setupStateDB() error {
	var (
		config       = s.config
		logger       = s.logger
		levelOptions = config.LeveldbOptions
	)

	db, err := leveldb.New(
		filepath.Join(config.DataDir, StateDataDir),
		leveldb.SetBloomKeyBits(levelOptions.BloomKeyBits),
		// trie cache + blockchain cache = leveldbOptions.CacheSize
		leveldb.SetCacheSize(levelOptions.CacheSize/2),
		leveldb.SetCompactionTableSize(levelOptions.CompactionTableSize),
		leveldb.SetCompactionTotalSize(levelOptions.CompactionTotalSize),
		leveldb.SetHandles(levelOptions.Handles),
		leveldb.SetLogger(logger.Named("database").With("path", StateDataDir)),
		leveldb.SetNoSync(levelOptions.NoSync),
	)
	if err != nil {
		return err
	}

	s.trieDB = db

	st := itrie.NewStateDB(db, logger, s.serverMetrics.trie)
	s.state = st

	return nil
}

func (s *Server) setupSnpTrieDB(trieDB kvdb.Database) {
	cacheConfig := s.cacheConfig

	s.snpTrieDB = trie.NewDatabaseWithConfig(
		trieDB,
		&trie.Config{
			Cache:   cacheConfig.TrieCleanLimit,
			Journal: cacheConfig.TrieCleanJournal,
		},
		s.logger.Named("snapshotTrieDB"),
	)
}

func (s *Server) setupBlockchainDB() error {
	var (
		config         = s.config
		leveldbOptions = config.LeveldbOptions
		logger         = s.logger
	)

	db, err := leveldb.New(
		filepath.Join(config.DataDir, BlockchainDataDir),
		leveldb.SetBloomKeyBits(leveldbOptions.BloomKeyBits),
		// trie cache + blockchain cache = leveldbOptions.CacheSize
		leveldb.SetCacheSize(leveldbOptions.CacheSize/2),
		leveldb.SetCompactionTableSize(leveldbOptions.CompactionTableSize),
		leveldb.SetCompactionTotalSize(leveldbOptions.CompactionTotalSize),
		leveldb.SetHandles(leveldbOptions.Handles),
		leveldb.SetLogger(logger.Named("database").With("path", BlockchainDataDir)),
		leveldb.SetNoSync(leveldbOptions.NoSync),
	)
	if err != nil {
		return err
	}

	s.chainDB = db

	return nil
}

// setupNetwork set up a libp2p network for the server
func (s *Server) setupNetwork() error {
	config := s.config
	netConfig := config.Network

	// more detail configuration
	netConfig.Chain = s.config.Chain
	netConfig.DataDir = filepath.Join(s.config.DataDir, "libp2p")
	netConfig.SecretsManager = s.secretsManager
	netConfig.Metrics = s.serverMetrics.network

	network, err := network.NewServer(s.logger, netConfig)
	if err != nil {
		return err
	}

	s.network = network

	return nil
}

// setupMetris set up metrics and metric server
//
// Must done before other components
func (s *Server) setupMetris() {
	var (
		config    = s.config
		namespace = "dogechain"
		chainName = config.Chain.Name
	)

	if config.Telemetry.PrometheusAddr == nil {
		s.serverMetrics = metricProvider(namespace, chainName, false, false)

		return
	}

	s.serverMetrics = metricProvider(namespace, chainName, true, config.Telemetry.EnableIOMetrics)
	s.prometheusServer = s.startPrometheusServer(config.Telemetry.PrometheusAddr)
}

func (s *Server) restoreChain() error {
	if s.config.RestoreFile == nil {
		return nil
	}

	if err := archive.RestoreChain(s.logger, s.blockchain, *s.config.RestoreFile); err != nil {
		return err
	}

	return nil
}

type txpoolHub struct {
	snaps *snapshot.Tree
	state state.State
	*blockchain.Blockchain
}

func (t *txpoolHub) GetNonce(root types.Hash, addr types.Address) uint64 {
	account, err := getCommittedAccount(t.snaps, t.state, root, addr)
	if err != nil {
		return 0
	}

	return account.Nonce
}

func (t *txpoolHub) GetBalance(root types.Hash, addr types.Address) (*big.Int, error) {
	account, err := getCommittedAccount(t.snaps, t.state, root, addr)
	if err != nil {
		return big.NewInt(0), err
	}

	return account.Balance, nil
}

// setupSecretsManager sets up the secrets manager
func (s *Server) setupSecretsManager() error {
	secretsManagerConfig := s.config.SecretsManager
	if secretsManagerConfig == nil {
		// No config provided, use default
		secretsManagerConfig = &secrets.SecretsManagerConfig{
			Type: secrets.Local,
		}
	}

	secretsManagerType := secretsManagerConfig.Type
	secretsManagerParams := &secrets.SecretsManagerParams{
		Logger: s.logger,
	}

	if secretsManagerType == secrets.Local {
		// The base directory is required for the local secrets manager
		secretsManagerParams.Extra = map[string]interface{}{
			secrets.Path: s.config.DataDir,
		}

		// When server started as daemon,
		// ValidatorKey is required for the local secrets manager
		if s.config.Daemon {
			secretsManagerParams.DaemonValidatorKey = s.config.ValidatorKey
			secretsManagerParams.IsDaemon = s.config.Daemon
		}
	}

	// Grab the factory method
	secretsManagerFactory, ok := GetSecretsManager(secretsManagerType)
	if !ok {
		return fmt.Errorf("secrets manager type '%s' not found", secretsManagerType)
	}

	// Instantiate the secrets manager
	secretsManager, factoryErr := secretsManagerFactory(
		secretsManagerConfig,
		secretsManagerParams,
	)

	if factoryErr != nil {
		return fmt.Errorf("unable to instantiate secrets manager, %w", factoryErr)
	}

	s.secretsManager = secretsManager

	return nil
}

// setupConsensus sets up the consensus mechanism
func (s *Server) setupConsensus() error {
	engineName := s.config.Chain.Params.GetEngine()
	engine, ok := GetConsensusBackend(engineName)

	if !ok {
		return fmt.Errorf("consensus engine '%s' not found", engineName)
	}

	engineConfig, ok := s.config.Chain.Params.Engine[engineName].(map[string]interface{})
	if !ok {
		engineConfig = map[string]interface{}{}
	}

	config := &consensus.Config{
		Params: s.config.Chain.Params,
		Config: engineConfig,
		Path:   filepath.Join(s.config.DataDir, "consensus"),
	}

	consensus, err := engine(
		&consensus.ConsensusParams{
			Context:        context.Background(),
			Seal:           s.config.Seal,
			Config:         config,
			Txpool:         s.txpool,
			Network:        s.network,
			Blockchain:     s.blockchain,
			Executor:       s.executor,
			Grpc:           s.grpcServer,
			Logger:         s.logger.Named("consensus"),
			Metrics:        s.serverMetrics.consensus,
			SecretsManager: s.secretsManager,
			BlockTime:      s.config.BlockTime,
			BlockBroadcast: s.config.BlockBroadcast,
		},
	)

	if err != nil {
		return err
	}

	s.consensus = consensus
	s.blockchain.SetConsensus(consensus)

	return nil
}

// SETUP //

// setupJSONRCP sets up the JSONRPC server, using the set configuration
func (s *Server) setupJSONRPC() error {
	hub := NewJSONRPCStore(
		s.snaps,
		s.state,
		s.blockchain,
		s.restoreProgression,
		s.txpool,
		s.executor,
		s.consensus,
		s.network,
		s.serverMetrics.jsonrpcStore,
	)

	// format the jsonrpc endpoint namespaces
	namespaces := make([]jsonrpc.Namespace, len(s.config.JSONRPC.JSONNamespace))
	for i, s := range s.config.JSONRPC.JSONNamespace {
		namespaces[i] = jsonrpc.Namespace(s)
	}

	conf := &jsonrpc.Config{
		Store:                    hub,
		Addr:                     s.config.JSONRPC.JSONRPCAddr,
		ChainID:                  uint64(s.config.Chain.Params.ChainID),
		AccessControlAllowOrigin: s.config.JSONRPC.AccessControlAllowOrigin,
		BatchLengthLimit:         s.config.JSONRPC.BatchLengthLimit,
		BlockRangeLimit:          s.config.JSONRPC.BlockRangeLimit,
		JSONNamespaces:           namespaces,
		EnableWS:                 s.config.JSONRPC.EnableWS,
		PriceLimit:               s.config.PriceLimit,
		EnablePProf:              s.config.JSONRPC.EnablePprof,
		Metrics:                  s.serverMetrics.jsonrpc,
	}

	srv, err := jsonrpc.NewJSONRPC(s.logger, conf)
	if err != nil {
		return err
	}

	s.jsonrpcServer = srv

	return nil
}

// setupGraphQL sets up the graphql server, using the set configuration
func (s *Server) setupGraphQL() error {
	if !s.config.EnableGraphQL {
		return nil
	}

	hub := NewJSONRPCStore(
		s.snaps,
		s.state,
		s.blockchain,
		s.restoreProgression,
		s.txpool,
		s.executor,
		s.consensus,
		s.network,
		s.serverMetrics.jsonrpcStore,
	)

	conf := &graphql.Config{
		Store:                    hub,
		Addr:                     s.config.GraphQL.GraphQLAddr,
		ChainID:                  uint64(s.config.Chain.Params.ChainID),
		AccessControlAllowOrigin: s.config.GraphQL.AccessControlAllowOrigin,
		BlockRangeLimit:          s.config.GraphQL.BlockRangeLimit,
		EnablePProf:              s.config.GraphQL.EnablePprof,
	}

	srv, err := graphql.NewGraphQLService(s.logger, conf)
	if err != nil {
		return err
	}

	s.graphqlServer = srv

	return nil
}

// setupGRPC sets up the grpc server and listens on tcp
func (s *Server) setupGRPC() error {
	proto.RegisterSystemServer(s.grpcServer, &systemService{server: s})

	lis, err := net.Listen("tcp", s.config.GRPCAddr.String())
	if err != nil {
		return err
	}

	go func() {
		if err := s.grpcServer.Serve(lis); err != nil {
			s.logger.Error(err.Error())
		}
	}()

	s.logger.Info("GRPC server running", "addr", s.config.GRPCAddr.String())

	return nil
}

// Chain returns the chain object of the client
func (s *Server) Chain() *chain.Chain {
	return s.config.Chain
}

// JoinPeer attempts to add a new peer to the networking server
func (s *Server) JoinPeer(rawPeerMultiaddr string, static bool) error {
	return s.network.JoinPeer(rawPeerMultiaddr, static)
}

// Close closes the server
// sequence:
//
//	consensus:
//		wait for consensus exit any IbftState,
//		stop write any block to blockchain storage and
//		stop write any state to state storage
//
//	txpool: stop accepting new transactions
//	networking: stop transport
//	stateStorage: safe close state storage
//	blockchain: safe close state storage
func (s *Server) Close() {
	s.logger.Info("close consensus layer")

	// Close the consensus layer
	if err := s.consensus.Close(); err != nil {
		s.logger.Error("failed to close consensus", "err", err)
	}

	// close the txpool's main loop
	if s.txpool != nil {
		s.logger.Info("close txpool")
		s.txpool.Close()
	}

	// Close the networking layer
	if s.network != nil {
		s.logger.Info("close network layer")

		if err := s.network.Close(); err != nil {
			s.logger.Error("failed to close networking", "err", err)
		}
	}

	// Journal state snapshot and flush caches
	s.journalSnapshots()

	if s.blockchain != nil {
		s.logger.Info("close blockchain")

		// Close the blockchain layer
		if err := s.blockchain.Close(); err != nil {
			s.logger.Error("failed to close blockchain", "err", err)
		}
	}

	// Close jsonrpc server
	if s.jsonrpcServer != nil {
		s.logger.Info("close rpc server")

		if err := s.jsonrpcServer.Close(); err != nil {
			s.logger.Error("failed to close jsonrpc server", "err", err)
		}
	}

	// Close graphql server
	if s.graphqlServer != nil {
		s.logger.Info("close graphql server")

		if err := s.graphqlServer.Close(); err != nil {
			s.logger.Error("failed to close graphql server", "err", err)
		}
	}

	// Close the state storage
	if s.trieDB != nil {
		s.logger.Info("close state storage")

		if err := s.trieDB.Close(); err != nil {
			s.logger.Error("failed to close storage for trie", "err", err)
		}
	}

	if s.chainDB != nil {
		s.logger.Info("close blockchain storage")

		// Close the blockchain storage
		if err := s.chainDB.Close(); err != nil {
			s.logger.Error("failed to close storage for blockchain", "err", err)
		}
	}

	if s.prometheusServer != nil {
		if err := s.prometheusServer.Shutdown(context.Background()); err != nil {
			s.logger.Error("Prometheus server shutdown error", err)
		}
	}
}

func (s *Server) journalSnapshots() {
	// Snapshot base layer root, used in journalling when restarted.
	var snapBase types.Hash
	// Ensure that the entirety of the state snapshot is journalled to disk.
	if s.snaps != nil {
		s.logger.Info("Journal state snapshots to disk")

		var err error
		if snapBase, err = s.snaps.Journal(s.blockchain.Header().StateRoot); err != nil {
			s.logger.Error("failed to journal state snapshot", "err", err)
		}
	}

	// Ensure the state of a recent block is also stored to disk before exiting.
	// We're writing different states to catch different restart scenarios:
	//  - HEAD:     So we don't need to reprocess any blocks in the general case
	//  - HEAD-1:   So we don't do large reorgs if our HEAD becomes an uncle
	//  - HEAD-127: So we have a hard limit on the number of blocks reexecuted
	//
	// Disable it in "archive" mode only
	// if !s.cacheConfig.TrieDirtyDisabled
	triedb := s.snpTrieDB

	for _, offset := range []uint64{0, 1, TriesInMemory - 1} {
		if number := s.blockchain.Header().Number; number > offset {
			recent, ok := s.blockchain.GetBlockByNumber(number-offset, true)
			if !ok {
				s.logger.Error("block not exists", "number", number-offset)

				continue
			}

			s.logger.Info("Writing cached state to disk",
				"block", recent.Number(),
				"hash", recent.Hash(),
				"root", recent.Header.StateRoot,
			)

			if err := triedb.Commit(recent.Header.StateRoot, true, nil); err != nil {
				s.logger.Error("Failed to commit recent state trie", "err", err)
			}
		}
	}

	// Update snapshot state root
	if snapBase != types.ZeroHash {
		s.logger.Info("Writing snapshot state to disk", "root", snapBase)

		if err := triedb.Commit(snapBase, true, nil); err != nil {
			s.logger.Error("Failed to commit recent state trie", "err", err)
		}
	}

	// Ensure all live cached entries be saved into disk, so that we can skip
	// cache warmup when node restarts.
	if s.cacheConfig.TrieCleanJournal != "" {
		s.snpTrieDB.SaveCache(s.cacheConfig.TrieCleanJournal)
	}
}

func (s *Server) startPrometheusServer(listenAddr *net.TCPAddr) *http.Server {
	srv := &http.Server{
		Addr: listenAddr.String(),
		Handler: promhttp.InstrumentMetricHandler(
			prometheus.DefaultRegisterer, promhttp.HandlerFor(
				prometheus.DefaultGatherer,
				promhttp.HandlerOpts{},
			),
		),
		ReadHeaderTimeout: time.Minute,
	}

	go func() {
		s.logger.Info("Prometheus server started", "addr=", listenAddr.String())

		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("Prometheus HTTP server ListenAndServe", "err", err)
		}
	}()

	return srv
}

// helper functions

func addressHash(addr types.Address) types.Hash {
	return crypto.Keccak256Hash(addr.Bytes())
}

// getCommittedAccount is used for fetching (cached) account state from both TxPool and JSON-RPC
//
// an error means something bad happen
func getCommittedAccount(
	snaps *snapshot.Tree,
	state state.State,
	stateRoot types.Hash,
	addr types.Address,
) (*stypes.Account, error) {
	if snaps != nil { // avoid crash
		if snap := snaps.Snapshot(stateRoot); snap != nil { // snap found
			account, err := snap.Account(addressHash(addr))
			if account != nil { // account found
				return account, nil
			} else {
				// might not covered yet
				hclog.Default().Debug("get account from snapshot failed", "address", addr, "err", err)
			}
		}
	}

	// If the snapshot is unavailable or reading from it fails, load from the database.
	db, err := state.NewSnapshotAt(stateRoot)
	if err != nil {
		return nil, err
	}

	account, err := db.GetAccount(addr)
	if err != nil {
		return nil, err
	}

	if account == nil {
		// create an initialized account for querying
		account = &stypes.Account{
			Balance: new(big.Int),
		}
	}

	return account, nil
}

// getCommittedStorage is used for fetching (cached) storage from JSON-RPC
func getCommittedStorage(
	snaps *snapshot.Tree,
	stateDB state.State,
	stateRoot types.Hash,
	addr types.Address,
	slot types.Hash,
) (types.Hash, error) {
	if snaps != nil { // query cached snapshot
		if snap := snaps.Snapshot(stateRoot); snap != nil {
			// query it from cached snapshot
			enc, err := snap.Storage(crypto.Keccak256Hash(addr.Bytes()), crypto.Keccak256Hash(slot.Bytes()))
			if err == nil { // found
				return utils.StorageBytesToHash(enc)
			} else {
				// print out error
				hclog.Default().Debug("failed to get storage", "address", addr, "slot", slot, "err", err)
			}
		}
	}

	// get account at state root, so as to get the correct storage root
	account, err := getCommittedAccount(snaps, stateDB, stateRoot, addr)
	if err != nil {
		// something bad happen
		return types.Hash{}, err
	}

	// fast returns
	if account.StorageRoot == types.ZeroHash ||
		account.StorageRoot == types.EmptyRootHash {
		return types.Hash{}, nil
	}

	// make a snapshot at root
	db, err := stateDB.NewSnapshotAt(stateRoot)
	if err != nil {
		return types.Hash{}, err
	}

	// If the snapshot is unavailable or reading from it fails, load from the database.
	return db.GetStorage(addr, account.StorageRoot, slot)
}
