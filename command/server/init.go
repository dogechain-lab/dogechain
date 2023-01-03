package server

import (
	"errors"
	"fmt"
	"math"
	"net"
	"time"

	"github.com/dogechain-lab/dogechain/network/common"

	"github.com/dogechain-lab/dogechain/chain"
	"github.com/dogechain-lab/dogechain/command/helper"
	"github.com/dogechain-lab/dogechain/network"
	"github.com/dogechain-lab/dogechain/secrets"
	"github.com/dogechain-lab/dogechain/server"
	"github.com/dogechain-lab/dogechain/types"
)

var (
	errInvalidBlockTime       = errors.New("invalid block time specified")
	errDataDirectoryUndefined = errors.New("data directory not defined")
	errInvalidCacheSize       = errors.New("invalid cache size")
	errInvalidPercentage      = errors.New("invalid cache percentage")
)

func (p *serverParams) initConfigFromFile() error {
	var parseErr error

	if p.rawConfig, parseErr = readConfigFile(p.configPath); parseErr != nil {
		return parseErr
	}

	return nil
}

func (p *serverParams) initRawParams() error {
	if err := p.initBlockGasTarget(); err != nil {
		return err
	}

	if err := p.initSecretsConfig(); err != nil {
		return err
	}

	if err := p.initGenesisConfig(); err != nil {
		return err
	}

	if err := p.initDataDirLocation(); err != nil {
		return err
	}

	if err := p.initBlockTime(); err != nil {
		return err
	}

	if p.isDevMode {
		p.initDevMode()
	}

	p.initPeerLimits()
	p.initLogFileLocation()

	if err := p.initCacheConfigs(); err != nil {
		return err
	}

	return p.initAddresses()
}

func (p *serverParams) initCacheConfigs() error {
	if p.rawConfig.CacheConfig.Cache <= 0 {
		return errInvalidCacheSize
	}

	// snapshot cache
	if p.rawConfig.CacheConfig.SnapshotPercentage < 0 {
		return errInvalidPercentage
	} else {
		p.rawConfig.CacheConfig.SnapshotCache = p.rawConfig.CacheConfig.Cache *
			p.rawConfig.CacheConfig.SnapshotPercentage / 100
	}

	// trie clean cache
	if p.rawConfig.CacheConfig.TrieCleanPercentage < 0 {
		return errInvalidPercentage
	} else {
		p.rawConfig.CacheConfig.TrieCleanCache = p.rawConfig.CacheConfig.Cache *
			p.rawConfig.CacheConfig.TrieCleanPercentage / 100
	}

	// trie dirty cache
	p.rawConfig.CacheConfig.TrieDirtyCache = p.rawConfig.CacheConfig.Cache -
		p.rawConfig.CacheConfig.SnapshotCache - p.rawConfig.CacheConfig.TrieCleanCache

	// clean cache rejournal duration
	d, err := time.ParseDuration(p.rawConfig.CacheConfig.TrieCleanRejournalRaw)
	if err != nil {
		return err
	} else {
		p.rawConfig.CacheConfig.TrieCleanCacheRejournal = d
	}

	p.rawConfig.CacheConfig.TrieTimeout = 5 * time.Minute

	return nil
}

func (p *serverParams) initBlockTime() error {
	if p.rawConfig.BlockTime < 1 {
		return errInvalidBlockTime
	}

	return nil
}

func (p *serverParams) initDataDirLocation() error {
	if p.rawConfig.DataDir == "" {
		return errDataDirectoryUndefined
	}

	return nil
}

func (p *serverParams) initLogFileLocation() {
	if p.isLogFileLocationSet() {
		p.logFileLocation = p.rawConfig.LogFilePath
	}
}

func (p *serverParams) initBlockGasTarget() error {
	var parseErr error

	if p.blockGasTarget, parseErr = types.ParseUint64orHex(
		&p.rawConfig.BlockGasTarget,
	); parseErr != nil {
		return parseErr
	}

	return nil
}

func (p *serverParams) initSecretsConfig() error {
	if !p.isSecretsConfigPathSet() {
		return nil
	}

	var parseErr error

	if p.secretsConfig, parseErr = secrets.ReadConfig(
		p.rawConfig.SecretsConfigPath,
	); parseErr != nil {
		return fmt.Errorf("unable to read secrets config file, %w", parseErr)
	}

	return nil
}

func (p *serverParams) initGenesisConfig() error {
	var parseErr error

	if p.genesisConfig, parseErr = chain.Import(
		p.rawConfig.GenesisPath,
	); parseErr != nil {
		return parseErr
	}

	return nil
}

func (p *serverParams) initDevMode() {
	// Dev mode:
	// - disables peer discovery
	// - enables all forks
	p.rawConfig.ShouldSeal = true
	p.rawConfig.Network.NoDiscover = true
	p.genesisConfig.Params.Forks = chain.AllForksEnabled

	p.initDevConsensusConfig()
}

func (p *serverParams) initDevConsensusConfig() {
	if !p.isDevConsensus() {
		return
	}

	p.genesisConfig.Params.Engine = map[string]interface{}{
		string(server.DevConsensus): map[string]interface{}{
			"interval": p.devInterval,
		},
	}
}

func (p *serverParams) initPeerLimits() {
	if !p.isMaxPeersSet() && !p.isPeerRangeSet() {
		// No peer limits specified, use the default limits
		p.initDefaultPeerLimits()

		return
	}

	if p.isPeerRangeSet() {
		// Some part of the peer range is specified
		p.initUsingPeerRange()

		return
	}

	if p.isMaxPeersSet() {
		// The max peer value is specified, derive precise limits
		p.initUsingMaxPeers()

		return
	}
}

func (p *serverParams) initDefaultPeerLimits() {
	defaultNetworkConfig := network.DefaultConfig()

	p.rawConfig.Network.MaxPeers = defaultNetworkConfig.MaxPeers
	p.rawConfig.Network.MaxInboundPeers = defaultNetworkConfig.MaxInboundPeers
	p.rawConfig.Network.MaxOutboundPeers = defaultNetworkConfig.MaxOutboundPeers
}

func (p *serverParams) initUsingPeerRange() {
	defaultConfig := network.DefaultConfig()

	if p.rawConfig.Network.MaxInboundPeers == unsetPeersValue {
		p.rawConfig.Network.MaxInboundPeers = defaultConfig.MaxInboundPeers
	}

	if p.rawConfig.Network.MaxOutboundPeers == unsetPeersValue {
		p.rawConfig.Network.MaxOutboundPeers = defaultConfig.MaxOutboundPeers
	}

	p.rawConfig.Network.MaxPeers = p.rawConfig.Network.MaxInboundPeers + p.rawConfig.Network.MaxOutboundPeers
}

func (p *serverParams) initUsingMaxPeers() {
	p.rawConfig.Network.MaxOutboundPeers = int64(
		math.Floor(
			float64(p.rawConfig.Network.MaxPeers) * network.DefaultDialRatio,
		),
	)
	p.rawConfig.Network.MaxInboundPeers = p.rawConfig.Network.MaxPeers - p.rawConfig.Network.MaxOutboundPeers
}

func (p *serverParams) initAddresses() error {
	if err := p.initPrometheusAddress(); err != nil {
		return err
	}

	if err := p.initLibp2pAddress(); err != nil {
		return err
	}

	// need libp2p address to be set before initializing nat address
	if err := p.initNATAddress(); err != nil {
		return err
	}

	if err := p.initDNSAddress(); err != nil {
		return err
	}

	if err := p.initJSONRPCAddress(); err != nil {
		return err
	}

	if err := p.initGraphQLAddress(); err != nil {
		return err
	}

	return p.initGRPCAddress()
}

func (p *serverParams) initPrometheusAddress() error {
	if !p.isPrometheusAddressSet() {
		return nil
	}

	var parseErr error

	if p.prometheusAddress, parseErr = helper.ResolveAddr(
		p.rawConfig.Telemetry.PrometheusAddr,
		helper.AllInterfacesBinding,
	); parseErr != nil {
		return parseErr
	}

	p.prometheusIOMetrics = p.rawConfig.Telemetry.EnableIOTimer

	return nil
}

func (p *serverParams) initLibp2pAddress() error {
	var parseErr error

	if p.libp2pAddress, parseErr = helper.ResolveAddr(
		p.rawConfig.Network.Libp2pAddr,
		helper.LocalHostBinding,
	); parseErr != nil {
		return parseErr
	}

	return nil
}

func (p *serverParams) initNATAddress() error {
	if !p.isNATAddressSet() {
		return nil
	}

	var parseErr error
	p.natAddress, parseErr = net.ResolveTCPAddr("tcp", p.rawConfig.Network.NatAddr)

	if parseErr != nil {
		//compatible with no port setups
		fmt.Printf("%s, use libp2p port\n", parseErr)

		oldNatAddrCfg := net.ParseIP(p.rawConfig.Network.NatAddr)
		if oldNatAddrCfg != nil {
			p.natAddress, parseErr = net.ResolveTCPAddr("tcp",
				fmt.Sprintf("%s:%d", oldNatAddrCfg.String(), p.libp2pAddress.Port),
			)
			if parseErr == nil {
				return nil
			}
		}

		return errInvalidNATAddress
	}

	return nil
}

func (p *serverParams) initDNSAddress() error {
	if !p.isDNSAddressSet() {
		return nil
	}

	var parseErr error

	if p.dnsAddress, parseErr = common.MultiAddrFromDNS(
		p.rawConfig.Network.DNSAddr, p.libp2pAddress.Port,
	); parseErr != nil {
		return parseErr
	}

	return nil
}

func (p *serverParams) initJSONRPCAddress() error {
	var parseErr error

	if p.jsonRPCAddress, parseErr = helper.ResolveAddr(
		p.rawConfig.JSONRPCAddr,
		helper.AllInterfacesBinding,
	); parseErr != nil {
		return parseErr
	}

	return nil
}

func (p *serverParams) initGraphQLAddress() error {
	var parseErr error

	if p.graphqlAddress, parseErr = helper.ResolveAddr(
		p.rawConfig.GraphQLAddr,
		helper.LocalHostBinding,
	); parseErr != nil {
		return parseErr
	}

	return nil
}

func (p *serverParams) initGRPCAddress() error {
	var parseErr error

	if p.grpcAddress, parseErr = helper.ResolveAddr(
		p.rawConfig.GRPCAddr,
		helper.LocalHostBinding,
	); parseErr != nil {
		return parseErr
	}

	return nil
}
