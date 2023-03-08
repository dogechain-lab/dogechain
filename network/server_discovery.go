package network

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"net"
	"time"

	"github.com/dogechain-lab/dogechain/network/client"
	"github.com/dogechain-lab/dogechain/network/common"
	"github.com/dogechain-lab/dogechain/network/discovery"
	"github.com/dogechain-lab/dogechain/network/grpc"
	"github.com/dogechain-lab/dogechain/network/proto"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"

	helperCommon "github.com/dogechain-lab/dogechain/helper/common"
	ranger "github.com/libp2p/go-cidranger"
	kb "github.com/libp2p/go-libp2p-kbucket"
)

// GetRandomBootnode fetches a random bootnode that's currently
// NOT connected, if any
func (s *DefaultServer) GetRandomBootnode() *peer.AddrInfo {
	bootnodes := make([]*peer.AddrInfo, 0)

	for _, v := range s.bootnodes.getBootnodes() {
		// Check if the bootnode is not self
		// filter self peer from bootnodes
		if v.ID != s.host.ID() {
			bootnodes = append(bootnodes, v)
		}
	}

	if len(bootnodes) > 0 {
		randNum, _ := rand.Int(rand.Reader, big.NewInt(int64(len(bootnodes))))

		return bootnodes[randNum.Int64()]
	}

	return nil
}

// NewDiscoveryClient returns a new or existing discovery service client connection
func (s *DefaultServer) NewDiscoveryClient(peerID peer.ID) (client.DiscoveryClient, error) {
	// Check if there is an active stream connection already
	if protoStream := s.GetProtoClient(common.DiscProto, peerID); protoStream != nil {
		if discoveryClt, ok := protoStream.(client.DiscoveryClient); ok {
			return discoveryClt, nil
		}
	}

	// Create a new stream connection and save, only single object
	// close and clear only when the peer is disconnected
	protoStream, err := s.NewProtoConnection(common.DiscProto, peerID)
	if err != nil {
		return nil, err
	}

	// Save the stream connection
	clt := client.NewDiscoveryClient(
		s.logger,
		proto.NewDiscoveryClient(protoStream),
		protoStream,
	)
	s.SaveProtoClient(common.DiscProto, clt, peerID)

	return clt, nil
}

// AddToPeerStore adds peer information to the node's peer store,
// static node and bootnode addresses are added with permanent TTL
func (s *DefaultServer) AddToPeerStore(peerInfo *peer.AddrInfo) {
	ttl := peerstore.AddressTTL

	if s.IsStaticPeer(peerInfo.ID) || s.IsBootnode(peerInfo.ID) {
		ttl = peerstore.PermanentAddrTTL
	}

	// add all addresses to the peer store
	s.host.Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, ttl)
}

// RemoveFromPeerStore removes peer information from the node's peer store, ignoring static nodes and bootnodes
func (s *DefaultServer) RemoveFromPeerStore(peerID peer.ID) {
	span := s.tracer.Start("network.RemoveFromPeerStore")
	defer span.End()

	s.host.Peerstore().RemovePeer(peerID)
	s.host.Peerstore().ClearAddrs(peerID)
}

// GetPeerInfo fetches the information of a peer
func (s *DefaultServer) GetPeerInfo(peerID peer.ID) *peer.AddrInfo {
	info := s.host.Peerstore().PeerInfo(peerID)

	return &info
}

// GetRandomPeer fetches a random peer from the peers list
func (s *DefaultServer) GetRandomPeer() *peer.ID {
	s.peersLock.RLock()
	defer s.peersLock.RUnlock()

	if len(s.peers) < 1 {
		return nil
	}

	randNum, _ := rand.Int(
		rand.Reader,
		big.NewInt(int64(len(s.peers))),
	)

	randomPeerIndx := int(randNum.Int64())

	counter := 0
	for peerID := range s.peers {
		if randomPeerIndx == counter {
			return &peerID
		}

		counter++
	}

	return nil
}

// setupDiscovery Sets up the discovery service for the node
func (s *DefaultServer) setupDiscovery() error {
	// Set up a fresh routing table
	keyID := kb.ConvertPeerID(s.host.ID())

	routingTable, err := kb.NewRoutingTable(
		helperCommon.MaxInt(
			helperCommon.ClampInt64ToInt(
				s.config.MaxInboundPeers+s.config.MaxOutboundPeers)*2, // double the k-bucket size, storage more peers info
			defaultBucketSize,
		),
		keyID,
		time.Minute,
		s.host.Peerstore(),
		10*time.Second,
		nil,
	)
	if err != nil {
		return err
	}

	// Set the PeerAdded event handler
	routingTable.PeerAdded = func(p peer.ID) {
		// check peer is not connected and has free outbound connections
		if s.connectionCounts.HasFreeOutboundConn() && !s.HasPeer(p) {
			info := s.host.Peerstore().PeerInfo(p)
			// TODO: use DefaultServer.ctx replace context.Background()
			s.addToDialQueue(context.Background(), &info, common.PriorityRandomDial)
		}
	}

	// Set the PeerRemoved event handler
	routingTable.PeerRemoved = func(p peer.ID) {
		s.dialQueue.DeleteTask(p)
	}

	// Create ignore CIDR filter
	ignoreCIDR := func(list []*net.IPNet) ranger.Ranger {
		if len(list) == 0 {
			return nil
		}

		// Create a new CIDR set
		ignoreRange := ranger.NewPCTrieRanger()

		for _, cidr := range list {
			// Add the CIDR to the set
			if cidr != nil {
				ignoreRange.Insert(ranger.NewBasicRangerEntry(*cidr))
			}
		}

		return ignoreRange
	}(s.config.DiscoverIngoreCIDR)

	// Create an instance of the discovery service
	discoveryService := discovery.NewDiscoveryService(
		s,
		routingTable,
		ignoreCIDR,
		s.logger,
	)

	// Register a network event handler
	if subscribeErr := s.SubscribeFn(context.Background(), discoveryService.HandleNetworkEvent); subscribeErr != nil {
		return fmt.Errorf("unable to subscribe to network events, %w", subscribeErr)
	}

	// Register the actual discovery service as a valid protocol
	s.registerDiscoveryService(discoveryService)

	if !s.config.NoDiscover {
		// Make sure the discovery service has the bootnodes in its routing table,
		// and instantiates connections to them
		discoveryService.ConnectToBootnodes(s.bootnodes.getBootnodes())
		// Start the discovery service
		discoveryService.Start()
	}

	return nil
}

// registerDiscoveryService registers the discovery protocol to be available
func (s *DefaultServer) registerDiscoveryService(discovery *discovery.DiscoveryService) {
	grpcStream := grpc.NewGrpcStream(context.TODO())
	proto.RegisterDiscoveryServer(grpcStream.GrpcServer(), discovery)
	grpcStream.Serve()

	s.RegisterProtocol(common.DiscProto, grpcStream)

	// Set the discovery service reference
	s.discovery = discovery
}
