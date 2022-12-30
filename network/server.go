package network

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dogechain-lab/dogechain/network/common"
	"github.com/dogechain-lab/dogechain/network/dial"
	"github.com/dogechain-lab/dogechain/network/discovery"
	"github.com/dogechain-lab/dogechain/secrets"

	peerEvent "github.com/dogechain-lab/dogechain/network/event"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	rawGrpc "google.golang.org/grpc"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/libp2p/go-libp2p/p2p/security/noise"

	"github.com/hashicorp/go-hclog"
	"github.com/multiformats/go-multiaddr"
)

const (
	// peerOutboundBufferSize is the size of outbound messages to a peer buffers in go-libp2p-pubsub
	// we should have enough capacity of the queue
	// because we start dropping messages to a peer if the outbound queue is full
	peerOutboundBufferSize = 1024

	// validateBufferSize is the size of validate buffers in go-libp2p-pubsub
	// we should have enough capacity of the queue
	// because when queue is full, validation is throttled and new messages are dropped.
	validateBufferSize = subscribeOutputBufferSize * 2
)

const (
	defaultBucketSize = 20
	DefaultDialRatio  = 0.2

	DefaultLibp2pPort int = 1478

	MinimumBootNodes       int   = 1
	MinimumPeerConnections int64 = 1

	DefaultKeepAliveTimer = 10 * time.Second
	DefaultDialTimeout    = 30 * time.Second
)

var (
	ErrNoBootnodes  = errors.New("no bootnodes specified")
	ErrMinBootnodes = errors.New("minimum 1 bootnode is required")
)

type DefaultServer struct {
	logger hclog.Logger // the logger
	config *Config      // the base networking server configuration

	closeCh chan struct{}  // the channel used for closing the networking server
	closeWg sync.WaitGroup // the waitgroup used for closing the networking server

	host  host.Host             // the libp2p host reference
	addrs []multiaddr.Multiaddr // the list of supported (bound) addresses

	peers     map[peer.ID]*PeerConnInfo // map of all peer connections
	peersLock sync.RWMutex              // lock for the peer map

	metrics *Metrics // reference for metrics tracking

	dialQueue *dial.DialQueue // queue used to asynchronously connect to peers

	discovery *discovery.DiscoveryService // service used for discovering other peers

	protocols     map[string]Protocol // supported protocols
	protocolsLock sync.Mutex          // lock for the supported protocols map

	secretsManager secrets.SecretsManager // secrets manager for networking keys

	ps *pubsub.PubSub // reference to the networking PubSub service

	emitterPeerEvent event.Emitter // event emitter for listeners

	connectionCounts *ConnectionInfo

	bootnodes *bootnodesWrapper // reference of all bootnodes for the node

	staticnodes *staticnodesWrapper // reference of all static nodes for the node
}

// NewServer returns a new instance of the networking server
func NewServer(logger hclog.Logger, config *Config) (Server, error) {
	return newServer(logger, config)
}

func newServer(logger hclog.Logger, config *Config) (*DefaultServer, error) {
	logger = logger.Named("network")

	key, err := setupLibp2pKey(config.SecretsManager)
	if err != nil {
		return nil, err
	}

	listenAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d", config.Addr.IP.String(), config.Addr.Port))
	if err != nil {
		return nil, err
	}

	addrsFactory := func(addrs []multiaddr.Multiaddr) []multiaddr.Multiaddr {
		if config.NatAddr != nil {
			addr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d", config.NatAddr.IP.String(), config.NatAddr.Port))
			if err != nil {
				logger.Error("failed to create NAT address", "error", err)

				return addrs
			}

			addrs = []multiaddr.Multiaddr{addr}
		} else if config.DNS != nil {
			addrs = []multiaddr.Multiaddr{config.DNS}
		}

		return addrs
	}

	if config.MaxPeers == 0 {
		return nil, fmt.Errorf("max peers is 0, please set MaxInboundPeers and MaxOutboundPeers greater than 0")
	}

	if int(config.MaxPeers) < len(config.Chain.Bootnodes) {
		return nil, fmt.Errorf(
			"max peers (%d) is less than bootnodes (%d)",
			config.MaxPeers,
			len(config.Chain.Bootnodes),
		)
	}

	// use libp2p connection manager to manage the number of connections
	cm, err := connmgr.NewConnManager(
		len(config.Chain.Bootnodes)+1,          // minimum number of connections
		int(config.MaxPeers),                   // maximum number of connections
		connmgr.WithGracePeriod(2*time.Minute), // grace period before pruning connections
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create libp2p stack: %w", err)
	}

	host, err := libp2p.New(
		// Use noise as the encryption protocol
		libp2p.Security(noise.ID, noise.New),
		libp2p.ListenAddrs(listenAddr),
		libp2p.AddrsFactory(addrsFactory),
		libp2p.Identity(key),
		libp2p.ConnectionManager(cm),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create libp2p stack: %w", err)
	}

	emitter, err := host.EventBus().Emitter(new(peerEvent.PeerEvent))
	if err != nil {
		return nil, err
	}

	srv := &DefaultServer{
		logger:           logger,
		config:           config,
		host:             host,
		addrs:            host.Addrs(),
		peers:            make(map[peer.ID]*PeerConnInfo),
		metrics:          config.Metrics,
		dialQueue:        dial.NewDialQueue(),
		closeCh:          make(chan struct{}),
		emitterPeerEvent: emitter,
		protocols:        map[string]Protocol{},
		secretsManager:   config.SecretsManager,
		bootnodes: &bootnodesWrapper{
			bootnodeArr:       make([]*peer.AddrInfo, 0),
			bootnodesMap:      make(map[peer.ID]*peer.AddrInfo),
			bootnodeConnCount: 0,
		},
		connectionCounts: NewBlankConnectionInfo(
			config.MaxInboundPeers,
			config.MaxOutboundPeers,
		),
	}

	// start gossip protocol
	ps, err := pubsub.NewGossipSub(
		context.Background(),
		host,
		pubsub.WithPeerOutboundQueueSize(peerOutboundBufferSize),
		pubsub.WithValidateQueueSize(validateBufferSize),
	)
	if err != nil {
		return nil, err
	}

	srv.ps = ps

	return srv, nil
}

// HasFreeConnectionSlot checks if there are free connection slots in the specified direction [Thread safe]
func (s *DefaultServer) HasFreeConnectionSlot(direction network.Direction) bool {
	return s.connectionCounts.HasFreeConnectionSlot(direction)
}

// PeerConnInfo holds the connection information about the peer
type PeerConnInfo struct {
	Info peer.AddrInfo

	connDirections  map[network.Direction]bool
	protocolStreams map[string]*rawGrpc.ClientConn
}

// addProtocolStream adds a protocol stream
func (pci *PeerConnInfo) addProtocolStream(protocol string, stream *rawGrpc.ClientConn) {
	pci.protocolStreams[protocol] = stream
}

// cleanProtocolStreams clean and closes all protocol stream
func (pci *PeerConnInfo) cleanProtocolStreams() []error {
	errs := []error{}

	for _, stream := range pci.protocolStreams {
		if stream != nil {
			errs = append(errs, stream.Close())
		}
	}

	pci.protocolStreams = make(map[string]*rawGrpc.ClientConn)

	return errs
}

// getProtocolStream fetches the protocol stream, if any
func (pci *PeerConnInfo) getProtocolStream(protocol string) *rawGrpc.ClientConn {
	return pci.protocolStreams[protocol]
}

// setupLibp2pKey is a helper method for setting up the networking private key
func setupLibp2pKey(secretsManager secrets.SecretsManager) (crypto.PrivKey, error) {
	var key crypto.PrivKey

	if secretsManager.HasSecret(secrets.NetworkKey) {
		// The key is present in the secrets manager, read it
		networkingKey, readErr := ReadLibp2pKey(secretsManager)
		if readErr != nil {
			return nil, fmt.Errorf("unable to read networking private key from Secrets Manager, %w", readErr)
		}

		key = networkingKey
	} else {
		// The key is not present in the secrets manager, generate it
		libp2pKey, libp2pKeyEncoded, keyErr := GenerateAndEncodeLibp2pKey()
		if keyErr != nil {
			return nil, fmt.Errorf("unable to generate networking private key for Secrets Manager, %w", keyErr)
		}

		// Write the networking private key to disk
		if setErr := secretsManager.SetSecret(secrets.NetworkKey, libp2pKeyEncoded); setErr != nil {
			return nil, fmt.Errorf("unable to store networking private key to Secrets Manager, %w", setErr)
		}

		key = libp2pKey
	}

	return key, nil
}

// Start starts the networking services
func (s *DefaultServer) Start() error {
	s.logger.Info("LibP2P server running", "addr", common.AddrInfoToString(s.AddrInfo()))

	if setupErr := s.setupIdentity(); setupErr != nil {
		return fmt.Errorf("unable to setup identity, %w", setupErr)
	}

	// Parse the static node data
	if setupErr := s.setupStaticnodes(); setupErr != nil {
		return fmt.Errorf("unable to parse static node data, %w", setupErr)
	}

	// Set up the peer discovery mechanism if needed
	if !s.config.NoDiscover {
		// Parse the bootnode data
		if setupErr := s.setupBootnodes(); setupErr != nil {
			return fmt.Errorf("unable to parse bootnode data, %w", setupErr)
		}

		// Setup and start the discovery service
		if setupErr := s.setupDiscovery(); setupErr != nil {
			return fmt.Errorf("unable to setup discovery, %w", setupErr)
		}
	}

	go s.runDial()
	go s.keepAliveMinimumPeerConnections()
	go s.keepAliveStaticPeerConnections()

	// watch for disconnected peers
	s.host.Network().Notify(&network.NotifyBundle{
		DisconnectedF: func(net network.Network, conn network.Conn) {
			// Update the local connection metrics
			s.removePeer(conn.RemotePeer())
		},
	})

	return nil
}

// setupStaticnodes setup the static node's connections
func (s *DefaultServer) setupStaticnodes() error {
	if s.staticnodes == nil {
		s.staticnodes = newStaticnodesWrapper()
	}

	if s.config.Chain.Staticnodes == nil || len(s.config.Chain.Staticnodes) == 0 {
		return nil
	}

	for _, rawAddr := range s.config.Chain.Staticnodes {
		staticnode, err := common.StringToAddrInfo(rawAddr)
		if err != nil {
			s.logger.Error("failed to parse staticnode", "rawAddr", rawAddr, "err", err)

			continue
		}

		if staticnode.ID == s.host.ID() {
			s.logger.Warn("staticnode is self", "rawAddr", rawAddr)

			continue
		}

		s.staticnodes.addStaticnode(staticnode)
		s.markStaticPeer(staticnode)
	}

	return nil
}

// keepAliveStaticPeerConnections keeps the static node connections alive
func (s *DefaultServer) keepAliveStaticPeerConnections() {
	s.closeWg.Add(1)
	defer s.closeWg.Done()

	if s.staticnodes == nil || s.staticnodes.Len() == 0 {
		return
	}

	allConnected := false

	delay := time.NewTimer(DefaultKeepAliveTimer)
	defer delay.Stop()

	for {
		// If all the static nodes are connected, double the delay
		if allConnected {
			delay.Reset(DefaultKeepAliveTimer * 2)
		} else {
			delay.Reset(DefaultKeepAliveTimer)
		}

		select {
		case <-delay.C:
		case <-s.closeCh:
			return
		}

		if s.staticnodes == nil || s.staticnodes.Len() == 0 {
			return
		}

		allConnected = true

		s.staticnodes.rangeAddrs(func(add *peer.AddrInfo) bool {
			if s.host.Network().Connectedness(add.ID) == network.Connected {
				return true
			}

			if allConnected {
				allConnected = false
			}

			s.joinPeer(add)

			return true
		})
	}
}

// setupBootnodes sets up the node's bootnode connections
func (s *DefaultServer) setupBootnodes() error {
	// Check the bootnode config is present
	if s.config.Chain.Bootnodes == nil {
		return ErrNoBootnodes
	}

	// Check if at least one bootnode is specified
	if len(s.config.Chain.Bootnodes) < MinimumBootNodes {
		return ErrMinBootnodes
	}

	bootnodesArr := make([]*peer.AddrInfo, 0)
	bootnodesMap := make(map[peer.ID]*peer.AddrInfo)

	for _, rawAddr := range s.config.Chain.Bootnodes {
		bootnode, err := common.StringToAddrInfo(rawAddr)
		if err != nil {
			return fmt.Errorf("failed to parse bootnode %s: %w", rawAddr, err)
		}

		if bootnode.ID == s.host.ID() {
			s.logger.Info("Omitting bootnode with same ID as host", "id", bootnode.ID)

			continue
		}

		bootnodesArr = append(bootnodesArr, bootnode)
		bootnodesMap[bootnode.ID] = bootnode
	}

	// It's fine for the bootnodes field to be unprotected
	// at this point because it is initialized once (doesn't change),
	// and used only after this point
	s.bootnodes = &bootnodesWrapper{
		bootnodeArr:       bootnodesArr,
		bootnodesMap:      bootnodesMap,
		bootnodeConnCount: 0,
	}

	return nil
}

// keepAliveMinimumPeerConnections will attempt to make new connections
// if the active peer count is lesser than the specified limit.
func (s *DefaultServer) keepAliveMinimumPeerConnections() {
	s.closeWg.Add(1)
	defer s.closeWg.Done()

	delay := time.NewTimer(DefaultKeepAliveTimer)
	defer delay.Stop()

	for {
		delay.Reset(DefaultKeepAliveTimer)

		select {
		case <-delay.C:
		case <-s.closeCh:
			return
		}

		if s.PeerCount() >= MinimumPeerConnections {
			continue
		}

		if s.config.NoDiscover || !s.bootnodes.hasBootnodes() {
			// dial unconnected peer
			randPeer := s.GetRandomPeer()
			if randPeer != nil && !s.IsConnected(*randPeer) {
				s.addToDialQueue(s.GetPeerInfo(*randPeer), common.PriorityRandomDial)
			}
		} else if randomNode := s.GetRandomBootnode(); randomNode != nil {
			// dial random unconnected bootnode
			s.addToDialQueue(randomNode, common.PriorityRandomDial)
		}
	}
}

// runDial starts the networking server's dial loop.
// Essentially, the networking server monitors for any open connection slots
// and attempts to fill them as soon as they open up
func (s *DefaultServer) runDial() {
	s.closeWg.Add(1)
	defer s.closeWg.Done()

	// Create a channel to notify the dial loop of any new dial requests
	// not need close the channel, this channel is write non-blocking
	notifyCh := make(chan struct{}, 1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := s.SubscribeFn(ctx, func(event *peerEvent.PeerEvent) {
		// Only concerned about the listed event types
		switch event.Type {
		case
			peerEvent.PeerConnected,
			peerEvent.PeerFailedToConnect,
			peerEvent.PeerDisconnected,
			peerEvent.PeerDialCompleted, // @Yoshiki, not sure we need to monitor this event type here
			peerEvent.PeerAddedToDialQueue:
		default:
			return
		}

		select {
		case notifyCh <- struct{}{}:
		default:
		}
	}); err != nil {
		s.logger.Error(
			"Cannot instantiate an event subscription for the dial manager",
			"err",
			err,
		)

		// Failing to subscribe to network events is fatal since the
		// dial manager relies on the event subscription routine to function
		return
	}

	for {
		// TODO: Right now the dial task are done sequentially because Connect
		// is a blocking request. In the future we should try to make up to
		// maxDials requests concurrently
		for s.connectionCounts.HasFreeOutboundConn() {
			tt := s.dialQueue.PopTask()
			if tt == nil {
				// The dial queue is closed,
				// no further dial tasks are incoming
				return
			}

			peerInfo := tt.GetAddrInfo()

			s.logger.Debug(fmt.Sprintf("Dialing peer [%s] as local [%s]", peerInfo.String(), s.host.ID()))

			// Attempt to connect to the peer
			func() {
				connectCtx, cancel := context.WithTimeout(ctx, DefaultDialTimeout)
				defer cancel()

				s.Connect(connectCtx, *peerInfo)
			}()
		}

		// wait until there is a change in the state of a peer that
		// might involve a new dial slot available
		select {
		case <-notifyCh:
			s.logger.Debug("new peerEvent, next dial loop")
		case <-s.closeCh:
			return
		}
	}
}

func (s *DefaultServer) Connect(ctx context.Context, peerInfo peer.AddrInfo) error {
	if !s.IsConnected(peerInfo.ID) {
		// the connection process is async because it involves connection (here) +
		// the handshake done in the identity service.
		if err := s.host.Connect(ctx, peerInfo); err != nil {
			s.logger.Debug("failed to dial", "addr", peerInfo.String(), "err", err.Error())

			s.emitEvent(peerInfo.ID, peerEvent.PeerFailedToConnect)
		}
	}

	return nil
}

// PeerCount returns the number of connected peers [Thread safe]
func (s *DefaultServer) PeerCount() int64 {
	s.peersLock.RLock()
	defer s.peersLock.RUnlock()

	return int64(len(s.peers))
}

// Peers returns a copy of the networking server's peer connection info set.
// Only one (initial) connection (inbound OR outbound) per peer is contained [Thread safe]
func (s *DefaultServer) Peers() []*PeerConnInfo {
	s.peersLock.RLock()
	defer s.peersLock.RUnlock()

	peers := make([]*PeerConnInfo, 0)
	for _, connectionInfo := range s.peers {
		peers = append(peers, connectionInfo)
	}

	return peers
}

// hasPeer checks if the peer is present in the peers list [Thread safe]
func (s *DefaultServer) HasPeer(peerID peer.ID) bool {
	s.peersLock.RLock()
	defer s.peersLock.RUnlock()

	_, ok := s.peers[peerID]

	return ok
}

// IsConnected checks if the networking server is connected to a peer
func (s *DefaultServer) IsConnected(peerID peer.ID) bool {
	return s.host.Network().Connectedness(peerID) == network.Connected
}

// IsBootnode checks if the peer is a bootnode
func (s *DefaultServer) IsBootnode(peerID peer.ID) bool {
	return s.bootnodes.isBootnode(peerID)
}

// IsStaticPeer checks if the peer is a static peer
func (s *DefaultServer) IsStaticPeer(peerID peer.ID) bool {
	return s.staticnodes.isStaticnode(peerID)
}

// GetProtocols fetches the list of node-supported protocols
func (s *DefaultServer) GetProtocols(peerID peer.ID) ([]string, error) {
	return s.host.Peerstore().GetProtocols(peerID)
}

// removePeer removes a peer from the networking server's peer list,
// and updates relevant counters and metrics. It is called from the
// disconnection callback of the libp2p network bundle (when the connection is closed)
func (s *DefaultServer) removePeer(peerID peer.ID) {
	s.peersLock.Lock()
	defer s.peersLock.Unlock()

	s.logger.Info("remove peer", "id", peerID.String())

	if s.IsStaticPeer(peerID) {
		s.logger.Info("peer is static node, ignore", "id", peerID.String())

		return
	}

	// Remove the peer from the peers map
	connectionInfo, ok := s.peers[peerID]
	if !ok {
		// Peer is not present in the peers map
		s.logger.Warn(
			fmt.Sprintf("Attempted removing missing peer info %s", peerID),
		)

		return
	}

	// Delete the peer from the peers map
	delete(s.peers, peerID)

	// Update connection counters
	for connDirection, active := range connectionInfo.connDirections {
		if active {
			s.connectionCounts.UpdateConnCountByDirection(-1, connDirection)
			s.updateConnCountMetrics(connDirection)
			s.updateBootnodeConnCount(peerID, -1)
		}
	}

	s.metrics.SetTotalPeerCount(
		float64(len(s.peers)),
	)

	if errs := connectionInfo.cleanProtocolStreams(); len(errs) > 0 {
		for _, err := range errs {
			if err != nil {
				s.logger.Error("close protocol streams failed", "err", err)
			}
		}
	}

	// Emit the event alerting listeners
	s.emitEvent(peerID, peerEvent.PeerDisconnected)
}

// updateBootnodeConnCount attempts to update the bootnode connection count
// by delta if the action is valid [Thread safe]
func (s *DefaultServer) updateBootnodeConnCount(peerID peer.ID, delta int64) {
	if s.config.NoDiscover || !s.IsBootnode(peerID) {
		// If the discovery service is not running
		// or the peer is not a bootnode, there is no need
		// to update bootnode connection counters
		return
	}

	s.bootnodes.increaseBootnodeConnCount(delta)
}

// ForgetPeer disconnects, remove and forget peer to prevent broadcast discovery to other peers
//
// Cauction: take care of using this to ignore peer from store, which may break peer discovery
func (s *DefaultServer) ForgetPeer(peer peer.ID, reason string) {
	if s.IsStaticPeer(peer) {
		s.logger.Debug("forget peer not works for static node", "id", peer, "reason", reason)

		return
	}

	s.logger.Warn("forget peer", "id", peer, "reason", reason)

	s.DisconnectFromPeer(peer, reason)
	s.forgetPeer(peer)
}

func (s *DefaultServer) forgetPeer(peer peer.ID) {
	p := s.GetPeerInfo(peer)
	if p == nil || len(p.Addrs) == 0 { // already removed?
		s.logger.Info("peer already removed from store", "id", peer)

		return
	}

	s.logger.Info("remove peer from store", "id", peer)

	if s.discovery != nil {
		// remove peer from routing table
		s.discovery.RemovePeerFromRoutingTable(peer)
	}

	// remove peer from peer store
	s.RemoveFromPeerStore(p)

	// remove peer from peer list
	s.removePeer(peer)
}

// DisconnectFromPeer disconnects the networking server from the specified peer
func (s *DefaultServer) DisconnectFromPeer(peer peer.ID, reason string) {
	if !s.IsConnected(peer) {
		return
	}

	if s.IsStaticPeer(peer) {
		return
	}

	s.logger.Info("closing connection to peer", "id", peer, "reason", reason)

	// Remove the peer from the dial queue
	s.dialQueue.DeleteTask(peer)

	// Close the peer connection
	if closeErr := s.host.Network().ClosePeer(peer); closeErr != nil {
		s.logger.Error("unable to gracefully close peer connection", "err", closeErr)
	}
}

var (
	// Anything below 35s is prone to false timeouts, as seen from empirical test data
	// Github action runners are very slow, so we need to increase the timeout
	DefaultJoinTimeout   = 100 * time.Second
	DefaultBufferTimeout = DefaultJoinTimeout + time.Second*30
)

// JoinPeer attempts to add a new peer to the networking server
func (s *DefaultServer) JoinPeer(rawPeerMultiaddr string, static bool) error {
	// Parse the raw string to a MultiAddr format
	parsedMultiaddr, err := multiaddr.NewMultiaddr(rawPeerMultiaddr)
	if err != nil {
		return err
	}

	// Extract the peer info from the Multiaddr
	peerInfo, err := peer.AddrInfoFromP2pAddr(parsedMultiaddr)
	if err != nil {
		return err
	}

	if peerInfo.ID == s.host.ID() {
		return fmt.Errorf("cannot join self")
	}

	if static {
		s.staticnodes.addStaticnode(peerInfo)
		s.markStaticPeer(peerInfo)
	}

	// Mark the peer as ripe for dialing (async)
	s.joinPeer(peerInfo)

	return nil
}

// markStaticPeer marks the peer as a static peer
func (s *DefaultServer) markStaticPeer(peerInfo *peer.AddrInfo) {
	s.logger.Info("Marking peer as static", "peer", peerInfo.ID)

	s.AddToPeerStore(peerInfo)
	s.host.ConnManager().TagPeer(peerInfo.ID, "staticnode", 1000)
	s.host.ConnManager().Protect(peerInfo.ID, "staticnode")
}

// joinPeer creates a new dial task for the peer (for async joining)
func (s *DefaultServer) joinPeer(peerInfo *peer.AddrInfo) {
	s.logger.Info("Join request", "addr", peerInfo.String())

	// This method can be completely refactored to support some kind of active
	// feedback information on the dial status, and not just asynchronous updates.
	// For this feature to work, the networking server requires a flexible event subscription
	// manager that is configurable and cancelable at any point in time
	s.addToDialQueue(peerInfo, common.PriorityRequestedDial)
}

func (s *DefaultServer) Close() error {
	// close dial queue
	s.dialQueue.Close()

	if s.discovery != nil {
		s.discovery.Close()
	}

	// send close signal to all goroutines
	close(s.closeCh)

	// wait for all goroutines to finish
	s.closeWg.Wait()

	// close libp2p network layer
	return s.host.Close()
}

// SaveProtocolStream saves the protocol stream to the peer
// protocol stream reference [Thread safe]
func (s *DefaultServer) SaveProtocolStream(
	protocol string,
	stream *rawGrpc.ClientConn,
	peerID peer.ID,
) {
	s.peersLock.Lock()
	defer s.peersLock.Unlock()

	connectionInfo, ok := s.peers[peerID]
	if !ok {
		s.logger.Warn(
			fmt.Sprintf(
				"Attempted to save protocol %s stream for non-existing peer %s",
				protocol,
				peerID,
			),
		)

		return
	}

	connectionInfo.addProtocolStream(protocol, stream)
}

// NewProtoConnection opens up a new stream on the set protocol to the peer,
// and returns a reference to the connection
func (s *DefaultServer) NewProtoConnection(protocol string, peerID peer.ID) (*rawGrpc.ClientConn, error) {
	s.protocolsLock.Lock()
	defer s.protocolsLock.Unlock()

	s.logger.Debug("NewProtoConnection", "protocol", protocol, "peer", peerID)

	p, ok := s.protocols[protocol]
	if !ok {
		return nil, fmt.Errorf("protocol not found: %s", protocol)
	}

	stream, err := s.NewStream(protocol, peerID)
	if err != nil {
		return nil, err
	}

	// TODO: all connection use context background, need to be fixed
	return p.Client(context.Background(), stream), nil
}

func (s *DefaultServer) NewStream(proto string, id peer.ID) (network.Stream, error) {
	return s.host.NewStream(context.Background(), id, protocol.ID(proto))
}

// GetProtoStream returns an active protocol stream if present, otherwise
// it returns nil
func (s *DefaultServer) GetProtoStream(protocol string, peerID peer.ID) *rawGrpc.ClientConn {
	s.peersLock.Lock()
	defer s.peersLock.Unlock()

	connectionInfo, ok := s.peers[peerID]
	if !ok {
		return nil
	}

	return connectionInfo.getProtocolStream(protocol)
}

func (s *DefaultServer) RegisterProtocol(id string, p Protocol) {
	s.protocolsLock.Lock()
	defer s.protocolsLock.Unlock()

	s.protocols[id] = p
	s.wrapStream(id, p.Handler())
}

func (s *DefaultServer) wrapStream(id string, handle func(network.Stream)) {
	s.host.SetStreamHandler(protocol.ID(id), func(stream network.Stream) {
		peerID := stream.Conn().RemotePeer()
		s.logger.Debug("open stream", "protocol", id, "peer", peerID)

		handle(stream)
	})
}

func (s *DefaultServer) AddrInfo() *peer.AddrInfo {
	return &peer.AddrInfo{
		ID:    s.host.ID(),
		Addrs: s.addrs,
	}
}

func (s *DefaultServer) addToDialQueue(addr *peer.AddrInfo, priority common.DialPriority) {
	s.dialQueue.AddTask(addr, priority)
	s.emitEvent(addr.ID, peerEvent.PeerAddedToDialQueue)
}

func (s *DefaultServer) emitEvent(peerID peer.ID, peerEventType peerEvent.PeerEventType) {
	// POTENTIALLY BLOCKING
	if err := s.emitterPeerEvent.Emit(peerEvent.PeerEvent{
		PeerID: peerID,
		Type:   peerEventType,
	}); err != nil {
		s.logger.Info("failed to emit event", "peer", peerID, "type", peerEventType, "err", err)
	}
}

// SubscribeFn is a helper method to run subscription of PeerEvents
func (s *DefaultServer) SubscribeFn(ctx context.Context, handler func(evnt *peerEvent.PeerEvent)) error {
	raw, err := s.host.EventBus().Subscribe(new(peerEvent.PeerEvent))
	if err != nil {
		return err
	}

	s.closeWg.Add(1)

	go func() {
		defer s.closeWg.Done()
		defer raw.Close()

		for {
			select {
			case <-ctx.Done():
				return
			case <-s.closeCh:
				return
			case evnt := <-raw.Out():
				if e, ok := evnt.(peerEvent.PeerEvent); ok {
					handler(&e)
				}
			}
		}
	}()

	return nil
}

// updateConnCountMetrics updates the connection count metrics
func (s *DefaultServer) updateConnCountMetrics(direction network.Direction) {
	switch direction {
	case network.DirInbound:
		s.metrics.SetInboundConnectionsCount(
			float64(s.connectionCounts.GetInboundConnCount()),
		)
	case network.DirOutbound:
		s.metrics.SetOutboundConnectionsCount(
			float64(s.connectionCounts.GetOutboundConnCount()),
		)
	}
}

// updatePendingConnCountMetrics updates the pending connection count metrics
func (s *DefaultServer) updatePendingConnCountMetrics(direction network.Direction) {
	switch direction {
	case network.DirInbound:
		s.metrics.SetPendingInboundConnectionsCount(
			float64(s.connectionCounts.GetPendingInboundConnCount()),
		)
	case network.DirOutbound:
		s.metrics.SetPendingOutboundConnectionsCount(
			float64(s.connectionCounts.GetPendingOutboundConnCount()),
		)
	}
}
