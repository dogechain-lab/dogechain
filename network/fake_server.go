package network

import (
	"context"
	"errors"

	"github.com/dogechain-lab/dogechain/network/event"
	"github.com/libp2p/go-libp2p-core/peer"
	rawGrpc "google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type FakeTopic struct{}

func (t *FakeTopic) Publish(obj proto.Message) error {
	return nil
}

func (t *FakeTopic) Subscribe(handler func(obj interface{}, from peer.ID)) error {
	return nil
}

func (t *FakeTopic) Close() error {
	return nil
}

type FakeServer struct{}

func (s *FakeServer) AddrInfo() *peer.AddrInfo {
	return &peer.AddrInfo{}
}

func (s *FakeServer) Peers() []*PeerConnInfo {
	return []*PeerConnInfo{}
}

func (s *FakeServer) IsConnected(peerID peer.ID) bool {
	return false
}

func (s *FakeServer) SubscribeCh(context.Context) (<-chan *event.PeerEvent, error) {
	return make(chan *event.PeerEvent), nil
}

func (s *FakeServer) NewTopic(protoID string, obj proto.Message) (Topic, error) {
	return &FakeTopic{}, nil
}

func (s *FakeServer) RegisterProtocol(string, Protocol) {}

func (s *FakeServer) GetProtoStream(protocol string, peerID peer.ID) *rawGrpc.ClientConn {
	return nil
}

func (s *FakeServer) NewProtoConnection(protocol string, peerID peer.ID) (*rawGrpc.ClientConn, error) {
	return nil, errors.New("not implemented")
}

func (s *FakeServer) SaveProtocolStream(protocol string, stream *rawGrpc.ClientConn, peerID peer.ID) {
}

func (s *FakeServer) CloseProtocolStream(protocol string, peerID peer.ID) error {
	return nil
}

func (s *FakeServer) ForgetPeer(peer peer.ID, reason string) {}

func (s *FakeServer) Start() error { return nil }

func (s *FakeServer) Close() error { return nil }

func (s *FakeServer) JoinPeer(rawPeerMultiaddr string) error { return nil }

func (s *FakeServer) GetProtocols(peerID peer.ID) ([]string, error) {
	return []string{}, nil
}

func (s *FakeServer) GetPeerInfo(peerID peer.ID) *peer.AddrInfo {
	return nil
}
