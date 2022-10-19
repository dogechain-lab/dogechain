package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/dogechain-lab/dogechain/jsonrpc"
	"github.com/dogechain-lab/dogechain/network"
	"github.com/dogechain-lab/dogechain/txpool"
	"github.com/hashicorp/hcl"
)

// Config defines the server configuration params
type Config struct {
	GenesisPath              string     `json:"chain_config"`
	SecretsConfigPath        string     `json:"secrets_config"`
	DataDir                  string     `json:"data_dir"`
	BlockGasTarget           string     `json:"block_gas_target"`
	GRPCAddr                 string     `json:"grpc_addr"`
	JSONRPCAddr              string     `json:"jsonrpc_addr"`
	Telemetry                *Telemetry `json:"telemetry"`
	Network                  *Network   `json:"network"`
	ShouldSeal               bool       `json:"seal"`
	TxPool                   *TxPool    `json:"tx_pool"`
	LogLevel                 string     `json:"log_level"`
	RestoreFile              string     `json:"restore_file"`
	BlockTime                uint64     `json:"block_time_s"`
	Headers                  *Headers   `json:"headers"`
	LogFilePath              string     `json:"log_to"`
	EnableGraphQL            bool       `json:"enable_graphql"`
	GraphQLAddr              string     `json:"graphql_addr"`
	JSONRPCBatchRequestLimit uint64     `json:"json_rpc_batch_request_limit" yaml:"json_rpc_batch_request_limit"`
	JSONRPCBlockRangeLimit   uint64     `json:"json_rpc_block_range_limit" yaml:"json_rpc_block_range_limit"`
	JSONNamespace            []string   `json:"json_namespace" yaml:"json_namespace"`
	EnableWS                 bool       `json:"enable_ws"`
}

// Telemetry holds the config details for metric services.
type Telemetry struct {
	PrometheusAddr string `json:"prometheus_addr"`
}

// Network defines the network configuration params
type Network struct {
	NoDiscover       bool   `json:"no_discover"`
	Libp2pAddr       string `json:"libp2p_addr"`
	NatAddr          string `json:"nat_addr"`
	DNSAddr          string `json:"dns_addr"`
	MaxPeers         int64  `json:"max_peers,omitempty"`
	MaxOutboundPeers int64  `json:"max_outbound_peers,omitempty"`
	MaxInboundPeers  int64  `json:"max_inbound_peers,omitempty"`
}

// TxPool defines the TxPool configuration params
type TxPool struct {
	PriceLimit            uint64 `json:"price_limit"`
	MaxSlots              uint64 `json:"max_slots"`
	PruneTickSeconds      uint64 `json:"prune_tick_seconds"`
	PromoteOutdateSeconds uint64 `json:"promote_outdate_seconds"`
}

// Headers defines the HTTP response headers required to enable CORS.
type Headers struct {
	AccessControlAllowOrigins []string `json:"access_control_allow_origins"`
}

// minimum block generation time in seconds
const defaultBlockTime uint64 = 2

// DefaultConfig returns the default server configuration
func DefaultConfig() *Config {
	defaultNetworkConfig := network.DefaultConfig()

	return &Config{
		GenesisPath:    "./genesis.json",
		DataDir:        "./dogechain-chain",
		BlockGasTarget: "0x0", // Special value signaling the parent gas limit should be applied
		Network: &Network{
			NoDiscover:       defaultNetworkConfig.NoDiscover,
			MaxPeers:         defaultNetworkConfig.MaxPeers,
			MaxOutboundPeers: defaultNetworkConfig.MaxOutboundPeers,
			MaxInboundPeers:  defaultNetworkConfig.MaxInboundPeers,
		},
		Telemetry:  &Telemetry{},
		ShouldSeal: false,
		TxPool: &TxPool{
			PriceLimit:            0,
			MaxSlots:              txpool.DefaultMaxSlots,
			PruneTickSeconds:      txpool.DefaultPruneTickSeconds,
			PromoteOutdateSeconds: txpool.DefaultPromoteOutdateSeconds,
		},
		LogLevel:    "INFO",
		RestoreFile: "",
		BlockTime:   defaultBlockTime,
		Headers: &Headers{
			AccessControlAllowOrigins: []string{"*"},
		},
		LogFilePath:              "",
		EnableGraphQL:            false,
		JSONRPCBatchRequestLimit: jsonrpc.DefaultJSONRPCBatchRequestLimit,
		JSONRPCBlockRangeLimit:   jsonrpc.DefaultJSONRPCBlockRangeLimit,
		JSONNamespace:            []string{string(jsonrpc.NamespaceAll)},
		EnableWS:                 false,
	}
}

// readConfigFile reads the config file from the specified path, builds a Config object
// and returns it.
//
// Supported file types: .json, .hcl
func readConfigFile(path string) (*Config, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var unmarshalFunc func([]byte, interface{}) error

	switch {
	case strings.HasSuffix(path, ".hcl"):
		unmarshalFunc = hcl.Unmarshal
	case strings.HasSuffix(path, ".json"):
		unmarshalFunc = json.Unmarshal
	default:
		return nil, fmt.Errorf("suffix of %s is neither hcl nor json", path)
	}

	config := DefaultConfig()
	config.Network = new(Network)
	config.Network.MaxPeers = -1
	config.Network.MaxInboundPeers = -1
	config.Network.MaxOutboundPeers = -1

	if err := unmarshalFunc(data, config); err != nil {
		return nil, err
	}

	return config, nil
}
