package network

import (
	"context"

	"github.com/dogechain-lab/dogechain/network/event"
	"github.com/libp2p/go-libp2p-core/peer"
	rawGrpc "google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type Network interface {
	// AddrInfo returns Network Info
	AddrInfo() *peer.AddrInfo
	// Peers returns current connected peers
	Peers() []*PeerConnInfo
	// IsConnected returns the node is connecting to the peer associated with the given ID
	IsConnected(peerID peer.ID) bool
	// SubscribeCh returns a channel of peer event
	SubscribeCh(context.Context) (<-chan *event.PeerEvent, error)
	// NewTopic Creates New Topic for gossip
	NewTopic(protoID string, obj proto.Message) (Topic, error)
	// RegisterProtocol registers gRPC service
	RegisterProtocol(string, Protocol)
	// GetProtoStream returns an active protocol stream if present, otherwise
	// it returns nil
	GetProtoStream(protocol string, peerID peer.ID) *rawGrpc.ClientConn
	// NewProtoConnection opens up a new stream on the set protocol to the peer,
	// and returns a reference to the connection
	NewProtoConnection(protocol string, peerID peer.ID) (*rawGrpc.ClientConn, error)
	// SaveProtocolStream saves stream
	SaveProtocolStream(protocol string, stream *rawGrpc.ClientConn, peerID peer.ID)
	// CloseProtocolStream closes stream
	CloseProtocolStream(protocol string, peerID peer.ID) error
	// ForgetPeer disconnects, remove and forget peer to prevent broadcast discovery to other peers
	ForgetPeer(peer peer.ID, reason string)
}

type Server interface {
	Network

	Start() error
	// Stop stops the server
	Close() error
	// JoinPeer joins a peer to the network
	JoinPeer(rawPeerMultiaddr string) error
	// GetProtocols returns the list of protocols supported by the peer
	GetProtocols(peerID peer.ID) ([]string, error)
	// GetPeerInfo returns the peer info for the given peer ID
	GetPeerInfo(peerID peer.ID) *peer.AddrInfo
}
