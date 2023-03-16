package network

import (
	"context"
	"fmt"
	"math/big"

	"github.com/dogechain-lab/dogechain/network/client"
	"github.com/dogechain-lab/dogechain/network/common"
	"github.com/dogechain-lab/dogechain/network/grpc"
	"github.com/dogechain-lab/dogechain/network/identity"
	"github.com/dogechain-lab/dogechain/network/proto"

	"github.com/libp2p/go-libp2p-kbucket/keyspace"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"

	peerEvent "github.com/dogechain-lab/dogechain/network/event"
	kbucket "github.com/libp2p/go-libp2p-kbucket"
)

// NewIdentityClient returns a new identity service client connection
func (s *DefaultServer) NewIdentityClient(ctx context.Context, peerID peer.ID) (client.IdentityClient, error) {
	conn, err := s.NewProtoConnection(ctx, common.IdentityProto, peerID)
	if err != nil {
		return nil, err
	}

	return client.NewIdentityClient(
		s.logger,
		s.metrics.GetGrpcMetrics(),
		proto.NewIdentityClient(conn),
		conn,
	), nil
}

// AddPeer adds a new peer to the networking server's peer list,
// and updates relevant counters and metrics
func (s *DefaultServer) AddPeer(id peer.ID, direction network.Direction) {
	span := s.tracer.Start("network.AddPeer")
	defer span.End()

	s.logger.Info("Peer connected", "id", id.String())

	// Update the peer connection info
	if connectionExists := s.addPeerInfo(id, direction); connectionExists {
		// The peer connection information was already present in the networking
		// server, so no connection metrics should be updated further
		return
	}

	// Emit the event alerting listeners
	// WARNING: THIS CALL IS POTENTIALLY BLOCKING
	// UNDER HEAVY LOAD. IT SHOULD BE SUBSTITUTED
	// WITH AN EVENT SYSTEM THAT ACTUALLY WORKS
	s.emitEvent(span.Context(), id, peerEvent.PeerConnected)
}

// addPeerInfo updates the networking server's internal peer info table
// and returns a flag indicating if the same peer connection previously existed.
// In case the peer connection previously existed, this is a noop
func (s *DefaultServer) addPeerInfo(id peer.ID, direction network.Direction) bool {
	s.peersLock.Lock()
	defer s.peersLock.Unlock()

	connectionInfo, connectionExists := s.peers[id]
	if connectionExists && connectionInfo.existsConnDirection(direction) {
		// Check if this peer already has an active connection status (saved info).
		// There is no need to do further processing
		return true
	}

	// Check if the connection info is already initialized
	if !connectionExists {
		// Create a new record for the connection info
		connectionInfo = &PeerConnInfo{
			Info:           s.host.Peerstore().PeerInfo(id),
			connDirections: make(map[network.Direction]bool),
		}

		// update ttl
		s.host.Peerstore().UpdateAddrs(id, peerstore.TempAddrTTL, peerstore.PermanentAddrTTL)
	}

	// Save the connection info to the networking server
	connectionInfo.addConnDirection(direction)

	s.peers[id] = connectionInfo

	// Update connection counters
	s.connectionCounts.UpdateConnCountByDirection(1, direction)
	s.updateConnCountMetrics(direction)

	// Update the metric stats
	s.metrics.SetTotalPeerCount(
		float64(len(s.peers)),
	)

	return false
}

// UpdatePendingConnCount updates the pending connection count in the specified direction [Thread safe]
func (s *DefaultServer) UpdatePendingConnCount(delta int64, direction network.Direction) {
	s.connectionCounts.UpdatePendingConnCountByDirection(delta, direction)

	s.updatePendingConnCountMetrics(direction)
}

// EmitEvent emits a specified event to the networking server's event bus
func (s *DefaultServer) EmitEvent(ctx context.Context, event *peerEvent.PeerEvent) {
	s.emitEvent(ctx, event.PeerID, event.Type)
}

// setupIdentity sets up the identity service for the node
func (s *DefaultServer) setupIdentity() error {
	if s.identity != nil {
		return fmt.Errorf("identity service already initialized")
	}

	span := s.tracer.Start("network.setupIdentity")
	defer span.End()

	// Create an instance of the identity service
	s.identity = identity.NewIdentityService(
		s,
		s.logger,
		int64(s.config.Chain.Params.ChainID),
		s.host.ID(),
	)

	// Register the identity service protocol
	s.registerIdentityService(s.identity)

	// Register the network notify bundle handlers
	s.host.Network().Notify(s.identity.GetNotifyBundle())

	return nil
}

// registerIdentityService registers the identity service
func (s *DefaultServer) registerIdentityService(identityService *identity.IdentityService) {
	grpcStream := grpc.NewGrpcStream(context.TODO())
	proto.RegisterIdentityServer(grpcStream.GrpcServer(), identityService)
	grpcStream.Serve()

	s.RegisterProtocol(common.IdentityProto, grpcStream)
}

func (s *DefaultServer) GetPeerDistance(peerID peer.ID) *big.Int {
	nodeKey := keyspace.Key{Space: keyspace.XORKeySpace, Bytes: kbucket.ConvertPeerID(s.AddrInfo().ID)}
	peerKey := keyspace.Key{Space: keyspace.XORKeySpace, Bytes: kbucket.ConvertPeerID(peerID)}

	return nodeKey.Distance(peerKey)
}
