package network

import (
	"context"
	"errors"
	"sync"

	"github.com/dogechain-lab/dogechain/network/event"

	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/atomic"
	"google.golang.org/protobuf/proto"

	rawGrpc "google.golang.org/grpc"
)

// NonetworkTopic is a fake topic that does nothing
// only used for testing or offline mode
type NonetworkTopic struct{}

func (t *NonetworkTopic) Publish(obj proto.Message) error {
	return nil
}

func (t *NonetworkTopic) Subscribe(handler func(obj interface{}, from peer.ID)) error {
	return nil
}

func (t *NonetworkTopic) Close() error {
	return nil
}

// NonetworkServer is a fake server that does nothing
// only used for testing or offline mode
type NonetworkServer struct {
	sublock sync.Mutex

	isClose atomic.Bool
	sub     []chan *event.PeerEvent
}

func (s *NonetworkServer) AddrInfo() *peer.AddrInfo {
	return &peer.AddrInfo{}
}

func (s *NonetworkServer) Peers() []*PeerConnInfo {
	return []*PeerConnInfo{}
}

func (s *NonetworkServer) PeerCount() int64 {
	return 0
}

func (s *NonetworkServer) IsStaticPeer(peerID peer.ID) bool {
	return false
}

func (s *NonetworkServer) IsConnected(peerID peer.ID) bool {
	return false
}

func (s *NonetworkServer) SubscribeFn(context.Context, func(evnt *event.PeerEvent)) error {
	return nil
}

func (s *NonetworkServer) NewTopic(protoID string, obj proto.Message) (Topic, error) {
	return &NonetworkTopic{}, nil
}

func (s *NonetworkServer) RegisterProtocol(string, Protocol) {}

func (s *NonetworkServer) NewProtoConnection(
	ctx context.Context,
	protocol string,
	peerID peer.ID,
) (*rawGrpc.ClientConn, error) {
	return nil, errors.New("not implemented")
}

func (s *NonetworkServer) GetMetrics() *Metrics {
	return NilMetrics()
}

func (s *NonetworkServer) ForgetPeer(peer peer.ID, reason string) {}

func (s *NonetworkServer) Start() error {
	s.isClose.Store(false)

	return nil
}

func (s *NonetworkServer) Close() error {
	s.sublock.Lock()
	defer s.sublock.Unlock()

	if s.isClose.Load() {
		return nil
	}

	s.isClose.Store(true)

	for _, sub := range s.sub {
		close(sub)
	}

	s.sub = s.sub[:0]

	return nil
}

func (s *NonetworkServer) JoinPeer(rawPeerMultiaddr string, static bool) error { return nil }

func (s *NonetworkServer) HasPeer(peerID peer.ID) bool { return false }

func (s *NonetworkServer) GetProtocols(peerID peer.ID) ([]string, error) {
	return []string{}, nil
}

func (s *NonetworkServer) GetPeerInfo(peerID peer.ID) *peer.AddrInfo {
	return nil
}

func (s *NonetworkServer) DisconnectFromPeer(peer peer.ID, reason string) {}
