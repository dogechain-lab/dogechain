package network

import (
	"context"

	"github.com/dogechain-lab/dogechain/network/event"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	rawGrpc "google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type Network interface {
	// **Peer**

	// AddrInfo returns Network Info
	AddrInfo() *peer.AddrInfo
	// Peers returns current connected peers
	Peers() []*PeerConnInfo
	// PeerCount returns the number of connected peers
	PeerCount() int64
	// GetPeerInfo returns the peer info for the given peer ID
	GetPeerInfo(peerID peer.ID) *peer.AddrInfo
	// JoinPeer joins a peer to the network
	JoinPeer(rawPeerMultiaddr string, static bool) error
	// HasPeer returns true if the peer is connected
	HasPeer(peerID peer.ID) bool
	// IsStaticPeer returns true if the peer is a static peer
	IsStaticPeer(peerID peer.ID) bool
	// IsConnected returns the node is connecting to the peer associated with the given ID
	IsConnected(peerID peer.ID) bool
	// DisconnectFromPeer disconnects the networking server from the specified peer
	DisconnectFromPeer(peer peer.ID, reason string)
	// ForgetPeer disconnects, remove and forget peer to prevent broadcast discovery to other peers
	ForgetPeer(peer peer.ID, reason string)

	// **Topic**

	// NewTopic Creates New Topic for gossip
	NewTopic(protoID string, obj proto.Message) (Topic, error)
	// SubscribeFn subscribe of peer event
	SubscribeFn(ctx context.Context, handler func(evnt *event.PeerEvent)) error

	// **Protocol**

	// RegisterProtocol registers gRPC service
	RegisterProtocol(string, Protocol)
	// GetProtocols returns the list of protocols supported by the peer
	GetProtocols(peerID peer.ID) ([]string, error)
	// GetProtoStream returns an active protocol stream if present, otherwise
	// it returns nil
	GetProtoStream(protocol string, peerID peer.ID) *rawGrpc.ClientConn
	// NewProtoConnection opens up a new stream on the set protocol to the peer,
	// and returns a reference to the connection
	NewProtoConnection(protocol string, peerID peer.ID) (*rawGrpc.ClientConn, error)
	// SaveProtocolStream saves stream
	SaveProtocolStream(protocol string, stream *rawGrpc.ClientConn, peerID peer.ID)
}

type Protocol interface {
	Client(context.Context, network.Stream) *rawGrpc.ClientConn
	Handler() func(network.Stream)
}

type Server interface {
	Network

	// Start starts the server
	Start() error
	// Stop stops the server
	Close() error
}
