package grpc

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"

	"github.com/dogechain-lab/dogechain/helper/common"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	manet "github.com/multiformats/go-multiaddr/net"
	grpcPeer "google.golang.org/grpc/peer"
)

type GrpcStream struct {
	ctx           context.Context
	ctxCancel     context.CancelFunc
	ctxCancelOnce sync.Once

	streamCh   chan network.Stream
	grpcServer *grpc.Server
}

func NewGrpcStream(ctx context.Context) *GrpcStream {
	ctx, cancel := context.WithCancel(ctx)

	return &GrpcStream{
		ctx:       ctx,
		ctxCancel: cancel,
		streamCh:  make(chan network.Stream),
		grpcServer: grpc.NewServer(
			grpc.UnaryInterceptor(interceptor),
			grpc.MaxRecvMsgSize(common.MaxGrpcMsgSize),
			grpc.MaxSendMsgSize(common.MaxGrpcMsgSize)),
	}
}

type Context struct {
	context.Context
	PeerID peer.ID
}

// interceptor is the middleware function that wraps
// gRPC peer data to custom Dogechain-Lab Dogechain structures
func interceptor(
	ctx context.Context,
	req interface{},
	_ *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	// Grab the peer info from the connection
	contextPeer, ok := grpcPeer.FromContext(ctx)
	if !ok {
		return nil, errors.New("invalid type assertion for peer context")
	}

	// The peer address is expected to be wrapped in a custom
	// structure that contains the PeerID
	addr, ok := contextPeer.Addr.(*wrapLibp2pAddr)
	if !ok {
		return nil, errors.New("invalid type assertion")
	}

	// Wrap the extracted PeerID and the context
	// so the stream handler has access to the PeerID
	return handler(
		&Context{
			Context: ctx,
			PeerID:  addr.id,
		},
		req,
	)
}

func (g *GrpcStream) Client(ctx context.Context, stream network.Stream) *grpc.ClientConn {
	return WrapClient(ctx, stream)
}

func (g *GrpcStream) Serve() {
	go func() {
		_ = g.grpcServer.Serve(g)
	}()
}

func (g *GrpcStream) Handler() func(network.Stream) {
	return func(stream network.Stream) {
		select {
		case <-g.ctx.Done():
			return
		case g.streamCh <- stream:
		}
	}
}

func (g *GrpcStream) RegisterService(sd *grpc.ServiceDesc, ss interface{}) {
	g.grpcServer.RegisterService(sd, ss)
}

func (g *GrpcStream) GrpcServer() *grpc.Server {
	return g.grpcServer
}

// --- listener ---

func (g *GrpcStream) Accept() (net.Conn, error) {
	select {
	case <-g.ctx.Done():
		return nil, io.EOF
	case stream := <-g.streamCh:
		return &streamConn{Stream: stream}, nil
	}
}

// Addr implements the net.Listener interface
func (g *GrpcStream) Addr() net.Addr {
	return fakeLocalAddr()
}

func (g *GrpcStream) Close() error {
	g.ctxCancelOnce.Do(g.ctxCancel)

	return nil
}

// --- conn ---

func WrapClient(ctx context.Context, s network.Stream) *grpc.ClientConn {
	opts := grpc.WithContextDialer(func(ctx context.Context, peerIdStr string) (net.Conn, error) {
		return &streamConn{s}, nil
	})

	conn, err := grpc.DialContext(
		ctx,
		"",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(common.MaxGrpcMsgSize),
			grpc.MaxCallSendMsgSize(common.MaxGrpcMsgSize),
		),
		grpc.WithBlock(),
		opts)

	if err != nil {
		// TODO: this should not fail at all
		panic(err)
	}

	return conn
}

// streamConn represents a net.Conn wrapped to be compatible with net.conn
type streamConn struct {
	network.Stream
}

type wrapLibp2pAddr struct {
	id peer.ID
	net.Addr
}

// LocalAddr returns the local address.
func (c *streamConn) LocalAddr() net.Addr {
	addr, err := manet.ToNetAddr(c.Stream.Conn().LocalMultiaddr())
	if err != nil {
		return fakeRemoteAddr()
	}

	return &wrapLibp2pAddr{Addr: addr, id: c.Stream.Conn().LocalPeer()}
}

// RemoteAddr returns the remote address.
func (c *streamConn) RemoteAddr() net.Addr {
	addr, err := manet.ToNetAddr(c.Stream.Conn().RemoteMultiaddr())
	if err != nil {
		return fakeRemoteAddr()
	}

	return &wrapLibp2pAddr{Addr: addr, id: c.Stream.Conn().RemotePeer()}
}

// fakeLocalAddr returns a dummy local address.
func fakeLocalAddr() net.Addr {
	return &net.TCPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 0,
	}
}

// fakeRemoteAddr returns a dummy remote address.
func fakeRemoteAddr() net.Addr {
	return &net.TCPAddr{
		IP:   net.ParseIP("127.1.0.1"),
		Port: 0,
	}
}
