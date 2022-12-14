package server

import (
	"net"

	"github.com/hashicorp/go-hclog"

	"github.com/dogechain-lab/dogechain/chain"
	"github.com/dogechain-lab/dogechain/network"
	"github.com/dogechain-lab/dogechain/secrets"
)

const DefaultGRPCPort int = 9632
const DefaultJSONRPCPort int = 8545
const DefaultGraphQLPort int = 9898
const DefaultPprofPort int = 6060

// Config is used to parametrize the minimal client
type Config struct {
	Chain *chain.Chain

	JSONRPC       *JSONRPC
	EnableGraphQL bool
	GraphQL       *GraphQL
	GRPCAddr      *net.TCPAddr
	LibP2PAddr    *net.TCPAddr

	PriceLimit            uint64
	MaxSlots              uint64
	BlockTime             uint64
	PruneTickSeconds      uint64
	PromoteOutdateSeconds uint64

	Telemetry *Telemetry
	Network   *network.Config

	DataDir     string
	RestoreFile *string

	LeveldbOptions *LeveldbOptions

	Seal           bool
	SecretsManager *secrets.SecretsManagerConfig

	LogLevel    hclog.Level
	LogFilePath string

	Daemon       bool
	ValidatorKey string

	BlockBroadcast bool
}

// LeveldbOptions holds the leveldb options
type LeveldbOptions struct {
	CacheSize           int
	Handles             int
	BloomKeyBits        int
	CompactionTableSize int
	CompactionTotalSize int
	NoSync              bool
}

// Telemetry holds the config details for metric services
type Telemetry struct {
	PrometheusAddr  *net.TCPAddr
	EnableIOMetrics bool
}

// JSONRPC holds the config details for the JSON-RPC server
type JSONRPC struct {
	JSONRPCAddr              *net.TCPAddr
	AccessControlAllowOrigin []string
	BatchLengthLimit         uint64
	BlockRangeLimit          uint64
	JSONNamespace            []string
	EnableWS                 bool
	EnablePprof              bool
}

type GraphQL struct {
	GraphQLAddr              *net.TCPAddr
	AccessControlAllowOrigin []string
	BlockRangeLimit          uint64
	EnablePprof              bool
}
