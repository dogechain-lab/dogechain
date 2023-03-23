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
	"github.com/dogechain-lab/dogechain/helper/progress"
	"github.com/dogechain-lab/dogechain/jsonrpc"
	"github.com/dogechain-lab/dogechain/network"
	"github.com/dogechain-lab/dogechain/secrets"
	"github.com/dogechain-lab/dogechain/server/proto"
	"github.com/dogechain-lab/dogechain/state"
	itrie "github.com/dogechain-lab/dogechain/state/immutable-trie"
	"github.com/dogechain-lab/dogechain/state/runtime/evm"
	"github.com/dogechain-lab/dogechain/state/runtime/precompiled"
	"github.com/dogechain-lab/dogechain/txpool"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/hashicorp/go-hclog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
)

// Minimal is the central manager of the blockchain client
type Server struct {
	logger       hclog.Logger
	config       *Config
	state        state.State
	stateStorage itrie.Storage

	consensus consensus.Consensus

	// blockchain stack
	blockchain *blockchain.Blockchain
	chain      *chain.Chain

	// state executor
	executor *state.Executor

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
	loggerDomainName = "dogechain"
)

var dirPaths = []string{
	"blockchain",
	"trie",
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

func newLevelDBBuilder(logger hclog.Logger, config *Config, path string) kvdb.LevelDBBuilder {
	leveldbBuilder := kvdb.NewLevelDBBuilder(
		logger,
		path,
	)

	// trie cache + blockchain cache = config.LeveldbOptions.CacheSize / 2
	leveldbBuilder.SetCacheSize(config.LeveldbOptions.CacheSize / 2).
		SetHandles(config.LeveldbOptions.Handles).
		SetBloomKeyBits(config.LeveldbOptions.BloomKeyBits).
		SetCompactionTableSize(config.LeveldbOptions.CompactionTableSize).
		SetCompactionTotalSize(config.LeveldbOptions.CompactionTotalSize).
		SetNoSync(config.LeveldbOptions.NoSync)

	return leveldbBuilder
}

// NewServer creates a new Minimal server, using the passed in configuration
func NewServer(config *Config) (*Server, error) {
	logger, err := newLoggerFromConfig(config)
	if err != nil {
		return nil, fmt.Errorf("could not setup new logger instance, %w", err)
	}

	m := &Server{
		logger: logger,
		config: config,
		chain:  config.Chain,
		grpcServer: grpc.NewServer(
			grpc.MaxRecvMsgSize(common.MaxGrpcMsgSize),
			grpc.MaxSendMsgSize(common.MaxGrpcMsgSize),
		),
		restoreProgression: progress.NewProgressionWrapper(progress.ChainSyncRestore),
	}

	m.logger.Info("Data dir", "path", config.DataDir)

	// Generate all the paths in the dataDir
	if err := common.SetupDataDir(config.DataDir, dirPaths); err != nil {
		return nil, fmt.Errorf("failed to create data directories: %w", err)
	}

	if config.Telemetry.PrometheusAddr != nil {
		m.serverMetrics = metricProvider("dogechain", config.Chain.Name, true, config.Telemetry.EnableIOMetrics)
		m.prometheusServer = m.startPrometheusServer(config.Telemetry.PrometheusAddr)
	} else {
		m.serverMetrics = metricProvider("dogechain", config.Chain.Name, false, false)
	}

	// Set up the secrets manager
	if err := m.setupSecretsManager(); err != nil {
		return nil, fmt.Errorf("failed to set up the secrets manager: %w", err)
	}

	// start libp2p
	{
		netConfig := config.Network
		netConfig.Chain = m.config.Chain
		netConfig.DataDir = filepath.Join(m.config.DataDir, "libp2p")
		netConfig.SecretsManager = m.secretsManager
		netConfig.Metrics = m.serverMetrics.network

		network, err := network.NewServer(logger, netConfig)
		if err != nil {
			return nil, err
		}
		m.network = network
	}

	// start blockchain object
	stateStorage, err := func() (itrie.Storage, error) {
		leveldbBuilder := newLevelDBBuilder(
			logger,
			config,
			filepath.Join(m.config.DataDir, "trie"),
		)

		return itrie.NewLevelDBStorage(leveldbBuilder)
	}()

	if err != nil {
		return nil, err
	}

	m.stateStorage = stateStorage

	st := itrie.NewStateDB(stateStorage, logger, m.serverMetrics.trie)
	m.state = st

	m.executor = state.NewExecutor(config.Chain.Params, st, logger)
	m.executor.SetRuntime(precompiled.NewPrecompiled())
	m.executor.SetRuntime(evm.NewEVM())

	// compute the genesis root state
	genesisRoot, err := m.executor.WriteGenesis(config.Chain.Genesis.Alloc)
	if err != nil {
		return nil, err
	}

	config.Chain.Genesis.StateRoot = genesisRoot

	// create leveldb storageBuilder
	leveldbBuilder := newLevelDBBuilder(
		logger,
		config,
		filepath.Join(m.config.DataDir, "blockchain"),
	)

	// blockchain object
	m.blockchain, err = blockchain.NewBlockchain(
		logger,
		config.Chain,
		kvstorage.NewLevelDBStorageBuilder(logger, leveldbBuilder),
		nil,
		m.executor,
		m.serverMetrics.blockchain,
	)
	if err != nil {
		return nil, err
	}

	// TODO: refactor the design. Executor and blockchain should not rely on each other.
	m.executor.GetHash = m.blockchain.GetHashHelper

	{
		hub := &txpoolHub{
			state:      m.state,
			Blockchain: m.blockchain,
		}

		blackList := make([]types.Address, len(m.config.Chain.Params.BlackList))
		for i, a := range m.config.Chain.Params.BlackList {
			blackList[i] = types.StringToAddress(a)
		}

		destructiveContracts := make([]types.Address, len(m.config.Chain.Params.DestructiveContracts))
		for i, a := range m.config.Chain.Params.DestructiveContracts {
			destructiveContracts[i] = types.StringToAddress(a)
		}

		// start transaction pool
		m.txpool, err = txpool.NewTxPool(
			logger,
			m.chain.Params.Forks.At(0),
			hub,
			m.grpcServer,
			m.network,
			m.serverMetrics.txpool,
			&txpool.Config{
				Sealing:               m.config.Seal,
				MaxSlots:              m.config.MaxSlots,
				PriceLimit:            m.config.PriceLimit,
				PruneTickSeconds:      m.config.PruneTickSeconds,
				PromoteOutdateSeconds: m.config.PromoteOutdateSeconds,
				BlackList:             blackList,
				DDOSProtection:        m.config.Chain.Params.DDOSProtection,
				DestructiveContracts:  destructiveContracts,
			},
		)
		if err != nil {
			return nil, err
		}

		// use the eip155 signer
		signer := crypto.NewEIP155Signer(uint64(m.config.Chain.Params.ChainID))
		m.txpool.SetSigner(signer)
	}

	{
		// Setup consensus
		if err := m.setupConsensus(); err != nil {
			return nil, err
		}
		m.blockchain.SetConsensus(m.consensus)
	}

	// after consensus is done, we can mine the genesis block in blockchain
	// This is done because consensus might use a custom Hash function so we need
	// to wait for consensus because we do any block hashing like genesis
	if err := m.blockchain.ComputeGenesis(); err != nil {
		return nil, err
	}

	// initialize data in consensus layer
	if err := m.consensus.Initialize(); err != nil {
		return nil, err
	}

	// setup and start jsonrpc server
	if err := m.setupJSONRPC(); err != nil {
		return nil, err
	}

	// setup and start graphql server
	if err := m.setupGraphQL(); err != nil {
		return nil, err
	}

	// restore archive data before starting
	if err := m.restoreChain(); err != nil {
		return nil, err
	}

	// start consensus
	if err := m.consensus.Start(); err != nil {
		return nil, err
	}

	// setup and start grpc server
	if err := m.setupGRPC(); err != nil {
		return nil, err
	}

	if err := m.network.Start(); err != nil {
		return nil, err
	}

	m.txpool.Start()

	return m, nil
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
	state state.State
	*blockchain.Blockchain
}

func (t *txpoolHub) GetNonce(root types.Hash, addr types.Address) uint64 {
	account, err := getAccountImpl(t.state, root, addr)
	if err != nil {
		return 0
	}

	return account.Nonce
}

func (t *txpoolHub) GetBalance(root types.Hash, addr types.Address) (*big.Int, error) {
	account, err := getAccountImpl(t.state, root, addr)
	if err != nil {
		if errors.Is(err, jsonrpc.ErrStateNotFound) {
			// not exists, stop error propagation
			return big.NewInt(0), nil
		}

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

	return nil
}

// SETUP //

// setupJSONRCP sets up the JSONRPC server, using the set configuration
func (s *Server) setupJSONRPC() error {
	hub := NewJSONRPCStore(
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
	return s.chain
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
		s.logger.Error("failed to close consensus", "err", err.Error())
	}

	s.logger.Info("close txpool")

	// close the txpool's main loop
	s.txpool.Close()

	s.logger.Info("close network layer")

	// Close the networking layer
	if err := s.network.Close(); err != nil {
		s.logger.Error("failed to close networking", "err", err.Error())
	}

	s.logger.Info("close state storage")

	// Close the state storage
	if err := s.stateStorage.Close(); err != nil {
		s.logger.Error("failed to close storage for trie", "err", err.Error())
	}

	s.logger.Info("close blockchain storage")

	// Close the blockchain layer
	if err := s.blockchain.Close(); err != nil {
		s.logger.Error("failed to close blockchain", "err", err.Error())
	}

	if s.prometheusServer != nil {
		if err := s.prometheusServer.Shutdown(context.Background()); err != nil {
			s.logger.Error("Prometheus server shutdown error", err)
		}
	}
}

// Entry is a backend configuration entry
type Entry struct {
	Enabled bool
	Config  map[string]interface{}
}

// SetupDataDir sets up the dogechain data directory and sub-folders
func SetupDataDir(dataDir string, paths []string) error {
	if err := createDir(dataDir); err != nil {
		return fmt.Errorf("failed to create data dir: (%s): %w", dataDir, err)
	}

	for _, path := range paths {
		path := filepath.Join(dataDir, path)
		if err := createDir(path); err != nil {
			return fmt.Errorf("failed to create path: (%s): %w", path, err)
		}
	}

	return nil
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

// createDir creates a file system directory if it doesn't exist
func createDir(path string) error {
	_, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if os.IsNotExist(err) {
		if err := os.MkdirAll(path, os.ModePerm); err != nil {
			return err
		}
	}

	return nil
}

// getAccountImpl is used for fetching account state from both TxPool and JSON-RPC
func getAccountImpl(state state.State, root types.Hash, addr types.Address) (*state.Account, error) {
	snap, err := state.NewSnapshotAt(root)
	if err != nil {
		return nil, fmt.Errorf("unable to get snapshot for root '%s': %w", root, err)
	}

	account, err := snap.GetAccount(addr)
	if err != nil {
		return nil, err
	} else if account == nil {
		return nil, jsonrpc.ErrStateNotFound
	}

	return account, nil
}
