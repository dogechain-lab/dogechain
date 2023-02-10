package discovery

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/dogechain-lab/dogechain/network/common"
	"github.com/dogechain-lab/dogechain/network/event"
	"github.com/hashicorp/go-hclog"
	"github.com/libp2p/go-libp2p-core/network"

	"github.com/dogechain-lab/dogechain/network/grpc"
	"github.com/dogechain-lab/dogechain/network/proto"
	"github.com/libp2p/go-libp2p-core/peer"
	kb "github.com/libp2p/go-libp2p-kbucket"

	ranger "github.com/libp2p/go-cidranger"
)

const (
	// maxDiscoveryPeerReqCount is the max peer count that
	// can be requested from other peers
	maxDiscoveryPeerReqCount = 16

	// peerDiscoveryInterval is the interval at which other
	// peers are queried for their peer sets
	peerDiscoveryInterval = 5 * time.Second

	// bootnodeDiscoveryInterval is the interval at which
	// random bootnodes are dialed for their peer sets
	bootnodeDiscoveryInterval = 30 * time.Second

	maxDiscoveryPeerReqTimeout = 10 * time.Second
)

// networkingServer defines the base communication interface between
// any networking server implementation and the DiscoveryService
type networkingServer interface {
	// BOOTNODE QUERIES //

	// GetRandomBootnode fetches a random bootnode, if any
	GetRandomBootnode() *peer.AddrInfo

	// PROTOCOL MANIPULATION //

	// NewDiscoveryClient returns a discovery gRPC client connection
	NewDiscoveryClient(peerID peer.ID) (proto.DiscoveryClient, error)

	// PEER MANIPULATION //

	// IsBootnode returns true if the peer is a bootnode
	IsBootnode(peerID peer.ID) bool

	// IsStaticPeer returns true if the peer is a static peer
	IsStaticPeer(peerID peer.ID) bool

	// isConnected checks if the networking server is connected to a peer
	IsConnected(peerID peer.ID) bool

	// Connect attempts to connect to the specified peer
	Connect(context.Context, peer.AddrInfo) error

	// DisconnectFromPeer attempts to disconnect from the specified peer
	DisconnectFromPeer(peerID peer.ID, reason string)

	// AddToPeerStore adds a peer to the networking server's peer store
	AddToPeerStore(peerInfo *peer.AddrInfo)

	// RemoveFromPeerStore removes peer information from the server's peer store
	RemoveFromPeerStore(peerInfo *peer.AddrInfo)

	// GetPeerInfo fetches the peer information from the server's peer store
	GetPeerInfo(peerID peer.ID) *peer.AddrInfo

	// GetRandomPeer fetches a random peer from the server's peer store
	GetRandomPeer() *peer.ID

	// CONNECTION INFORMATION //

	// HasFreeConnectionSlot checks if there is an available connection slot for the set direction [Thread safe]
	HasFreeConnectionSlot(direction network.Direction) bool

	// PeerCount connection peer number
	PeerCount() int64
}

// DiscoveryService is a service that finds other peers in the network
// and connects them to the current running node
type DiscoveryService struct {
	proto.UnimplementedDiscoveryServer

	baseServer   networkingServer // The interface towards the base networking server
	logger       hclog.Logger     // The DiscoveryService logger
	routingTable *kb.RoutingTable // Kademlia 'k-bucket' routing table that contains connected nodes info

	ignoreCIDR ranger.Ranger // CIDR ranges to ignore when finding peers

	// ctx used for stopping the DiscoveryService
	ctx       context.Context
	ctxCancel context.CancelFunc
}

// NewDiscoveryService creates a new instance of the discovery service
func NewDiscoveryService(
	server networkingServer,
	routingTable *kb.RoutingTable,
	ignoreCIDR ranger.Ranger,
	logger hclog.Logger,
) *DiscoveryService {
	ctx, cancel := context.WithCancel(context.Background())

	return &DiscoveryService{
		baseServer:   server,
		logger:       logger.Named("discovery"),
		routingTable: routingTable,
		ignoreCIDR:   ignoreCIDR,
		ctx:          ctx,
		ctxCancel:    cancel,
	}
}

// Start starts the discovery service
func (d *DiscoveryService) Start() {
	go d.startDiscovery()
}

// Close stops the discovery service
func (d *DiscoveryService) Close() {
	d.ctxCancel()
}

// RoutingTableSize returns the size of the routing table
func (d *DiscoveryService) RoutingTableSize() int {
	return d.routingTable.Size()
}

// RoutingTablePeers fetches the peers from the routing table
func (d *DiscoveryService) RoutingTablePeers() []peer.ID {
	return d.routingTable.ListPeers()
}

// HandleNetworkEvent handles base network events for the DiscoveryService
func (d *DiscoveryService) HandleNetworkEvent(peerEvent *event.PeerEvent) {
	peerID := peerEvent.PeerID

	switch peerEvent.Type {
	case event.PeerConnected:
		// Add peer to the routing table and to our local peer table
		_, err := d.routingTable.TryAddPeer(peerID, false, false)
		if err != nil {
			d.logger.Error("failed to add peer to routing table", "err", err)

			return
		}
	case event.PeerDisconnected, event.PeerFailedToConnect:
		// Run cleanup for the local routing / reference peers table
		d.routingTable.RemovePeer(peerID)
	}
}

// ConnectToBootnodes attempts to connect to the bootnodes
// and add them to the peer / routing table
func (d *DiscoveryService) ConnectToBootnodes(bootnodes []*peer.AddrInfo) {
	for _, nodeInfo := range bootnodes {
		d.baseServer.AddToPeerStore(nodeInfo)

		if _, err := d.routingTable.TryAddPeer(nodeInfo.ID, true, false); err != nil {
			d.logger.Error(
				"Failed to add new peer to routing table",
				"peer",
				nodeInfo.ID,
				"err",
				err,
			)
		}
	}
}

// addToTable adds the node to the peer store and the routing table
func (d *DiscoveryService) addToTable(node *peer.AddrInfo) error {
	// before we include peers on the routing table -> dial queue
	// we have to add them to the peer store so that they are
	// available to all the libp2p services
	d.baseServer.AddToPeerStore(node)

	if _, err := d.routingTable.TryAddPeer(
		node.ID,
		false,
		true, // allow replacing existing peers
	); err != nil {
		// Since the routing table addition failed,
		// the peer can be removed from the libp2p peer store
		// in the base networking server
		d.baseServer.RemoveFromPeerStore(node)

		return err
	}

	return nil
}

func (d *DiscoveryService) RemovePeerFromRoutingTable(peerID peer.ID) {
	d.routingTable.RemovePeer(peerID)
}

// addPeersToTable adds the passed in peers to the peer store and the routing table
func (d *DiscoveryService) addPeersToTable(nodeAddrStrs []string) {
	for _, nodeAddrStr := range nodeAddrStrs {
		// Convert the string address info to a working type
		nodeInfo, err := common.StringToAddrInfo(nodeAddrStr)
		if err != nil {
			d.logger.Error(
				"Failed to parse address",
				"err",
				err,
			)

			continue
		}

		// ignore the peer if it is in the ignore CIDR range
		// this is used to ignore peers like only lan network address peer
		// but if the peer is a static peer, we should not ignore it
		if d.checkPeerInIgnoreCIDR(nodeAddrStr) && !d.baseServer.IsStaticPeer(nodeInfo.ID) {
			continue
		}

		if err := d.addToTable(nodeInfo); err != nil {
			d.logger.Error(
				"Failed to add new peer to routing table",
				"peer",
				nodeInfo.ID,
				"err",
				err,
			)
		}
	}
}

// attemptToFindPeers dials the specified peer and requests
// to see their peer list
func (d *DiscoveryService) attemptToFindPeers(peerID peer.ID) error {
	d.logger.Debug("Querying a peer for near peers", "peer", peerID)
	nodes, err := d.findPeersCall(peerID)

	d.logger.Debug("Found new near peers", "peer", len(nodes))
	d.addPeersToTable(nodes)

	return err
}

// checkPeerInIgnoreCIDR checks if the peer is in the ignore CIDR range
func (d *DiscoveryService) checkPeerInIgnoreCIDR(peerAddr string) bool {
	if d.ignoreCIDR == nil {
		return false
	}

	peerInfo, err := common.StringToAddrInfo(peerAddr)
	if err != nil || peerInfo == nil {
		// failed back to the default behaviour
		d.logger.Error("cant parse peer address", "err", err)

		return false
	}

	findValidAddress := false

	for _, addr := range peerInfo.Addrs {
		ip, err := common.ParseMultiaddrIP(addr)
		if err != nil && errors.Is(err, common.ErrMultiaddrContainsDNS) {
			d.logger.Debug("peer multiaddr is dns", "err", err)

			findValidAddress = true

			break
		}

		// Check if the peer IP is in the ignore CIDR range
		exist, err := d.ignoreCIDR.Contains(ip)
		if err == nil && exist {
			findValidAddress = true

			break
		}
	}

	return findValidAddress
}

// findPeersCall queries the set peer for their peer set
func (d *DiscoveryService) findPeersCall(
	peerID peer.ID,
) ([]string, error) {
	ctx, cancel := context.WithTimeout(d.ctx, maxDiscoveryPeerReqTimeout)
	defer cancel()

	clt, clientErr := d.baseServer.NewDiscoveryClient(peerID)
	if clientErr != nil {
		return nil, fmt.Errorf("unable to create new discovery client connection, %w", clientErr)
	}

	resp, err := clt.FindPeers(
		ctx,
		&proto.FindPeersReq{
			Count: maxDiscoveryPeerReqCount,
		},
	)
	if err != nil {
		return nil, err
	}

	var filterNode []string

	for _, node := range resp.Nodes {
		if !d.checkPeerInIgnoreCIDR(node) {
			filterNode = append(filterNode, node)
		}
	}

	return filterNode, nil
}

// startDiscovery starts the DiscoveryService loop,
// in which random peers are dialed for their peer sets,
// and random bootnodes are dialed for their peer sets
func (d *DiscoveryService) startDiscovery() {
	peerDiscoveryTicker := time.NewTicker(peerDiscoveryInterval)
	bootnodeDiscoveryTicker := time.NewTicker(bootnodeDiscoveryInterval)

	seed := time.Now().UnixNano()
	//#nosec G404
	r := rand.New(rand.NewSource(seed))

	defer func() {
		peerDiscoveryTicker.Stop()
		bootnodeDiscoveryTicker.Stop()
	}()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-peerDiscoveryTicker.C:
			// random delay to avoid all nodes querying at the same time
			peerDiscoveryTicker.Reset(
				peerDiscoveryInterval + time.Duration(r.Int63n(int64(peerDiscoveryInterval/2))),
			)

			go d.regularPeerDiscovery()
		case <-bootnodeDiscoveryTicker.C:
			// ditto
			bootnodeDiscoveryTicker.Reset(
				bootnodeDiscoveryInterval + time.Duration(r.Int63n(int64(bootnodeDiscoveryInterval/2))),
			)

			go d.bootnodePeerDiscovery()
		}
	}
}

// regularPeerDiscovery grabs a random peer from the list of
// connected peers, and attempts to find / connect to their peer set
func (d *DiscoveryService) regularPeerDiscovery() {
	if !d.baseServer.HasFreeConnectionSlot(network.DirOutbound) {
		// No need to do peer discovery if no open connection slots
		// are available
		return
	}

	// Grab a random peer from the base server's peer store
	peerID := d.baseServer.GetRandomPeer()
	if peerID == nil {
		// The node cannot find a random peer to query
		// from the current peer set
		return
	}

	d.logger.Debug("running regular peer discovery", "peer", peerID.String())
	// Try to discover the peers connected to the reference peer
	if err := d.attemptToFindPeers(*peerID); err != nil {
		d.logger.Error(
			"Failed to find new peers",
			"peer",
			peerID,
			"err",
			err,
		)
	}
}

// bootnodePeerDiscovery queries a random (unconnected) bootnode for new peers
// and adds them to the routing table
func (d *DiscoveryService) bootnodePeerDiscovery() {
	if !d.baseServer.HasFreeConnectionSlot(network.DirOutbound) {
		// No need to attempt bootnode dialing, since no
		// open outbound slots are left
		d.logger.Warn("no free connection slot, bootnode discovery failed")

		return
	}

	var bootnode *peer.AddrInfo // the reference bootnode

	// Try to find a suitable bootnode to use as a reference peer
	for bootnode == nil {
		// Get a random unconnected bootnode from the bootnode set
		bootnode = d.baseServer.GetRandomBootnode()
		if bootnode == nil {
			return
		}
	}

	// If bootnode is not connected try reonnect
	if !d.baseServer.IsConnected(bootnode.ID) {
		d.logger.Debug("bootnode is not connected, try to reconnect")

		connectCtx, cancel := context.WithTimeout(d.ctx, bootnodeDiscoveryInterval/2)
		defer cancel()

		if err := d.baseServer.Connect(connectCtx, *bootnode); err != nil {
			d.logger.Error("Unable to connect to bootnode",
				"bootnode", bootnode.ID.String(),
				"err", err.Error(),
			)

			return
		}
	}

	// Find peers from the referenced bootnode
	foundNodes, err := d.findPeersCall(bootnode.ID)
	if err != nil {
		d.logger.Error("Unable to execute bootnode peer discovery",
			"bootnode", bootnode.ID.String(),
			"err", err.Error(),
		)

		return
	}

	// Save the peers for subsequent dialing
	d.addPeersToTable(foundNodes)
}

// FindPeers implements the proto service for finding the target's peers
func (d *DiscoveryService) FindPeers(
	ctx context.Context,
	req *proto.FindPeersReq,
) (*proto.FindPeersResp, error) {
	// Extract the requesting peer ID from the gRPC context
	grpcContext, ok := ctx.(*grpc.Context)
	if !ok {
		return nil, errors.New("invalid type assertion")
	}

	from := grpcContext.PeerID

	// Sanity check for result set size
	if req.Count > maxDiscoveryPeerReqCount {
		req.Count = maxDiscoveryPeerReqCount
	}

	// The request Key is used for finding the closest peers
	// by utilizing Kademlia's distance calculation.
	// This way, the peer that's being queried for its peers delivers
	// only the closest ones to the requested key (peer)
	if req.GetKey() == "" {
		req.Key = from.String()
	}

	nearestPeers := d.routingTable.NearestPeers(
		kb.ConvertKey(req.GetKey()),
		int(req.Count)+1,
	)

	// The peer that's initializing this request
	// doesn't need to be a part of the resulting set
	filteredPeers := make([]string, 0)

	for _, id := range nearestPeers {
		if id == from {
			// Skip the peer that's initializing the request
			continue
		}

		if info := d.baseServer.GetPeerInfo(id); len(info.Addrs) > 0 {
			filteredPeers = append(filteredPeers, common.AddrInfoToString(info))
		}
	}

	return &proto.FindPeersResp{
		Nodes: filteredPeers,
	}, nil
}
