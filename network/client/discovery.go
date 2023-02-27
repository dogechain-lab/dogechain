//nolint:dupl
package client

import (
	"context"

	"github.com/dogechain-lab/dogechain/network/proto"
	"github.com/hashicorp/go-hclog"
	"go.uber.org/atomic"

	rawGrpc "google.golang.org/grpc"
)

type DiscoveryClient interface {
	GrpcClientCloser

	FindPeers(ctx context.Context, in *proto.FindPeersReq) (*proto.FindPeersResp, error)
}

type discoveryClient struct {
	clt  proto.DiscoveryClient
	conn *rawGrpc.ClientConn

	isClosed *atomic.Bool
}

func (i *discoveryClient) FindPeers(ctx context.Context, in *proto.FindPeersReq) (*proto.FindPeersResp, error) {
	return i.clt.FindPeers(ctx, in, rawGrpc.WaitForReady(false))
}

func (i *discoveryClient) Close() error {
	if i.isClosed.CompareAndSwap(false, true) {
		return i.conn.Close()
	} else {
		return ErrClientClosed
	}
}

func (i *discoveryClient) IsClose() bool {
	return i.isClosed.Load()
}

func NewDiscoveryClient(
	logger hclog.Logger,
	clt proto.DiscoveryClient,
	conn *rawGrpc.ClientConn,
) DiscoveryClient {
	wrapClt := &discoveryClient{
		clt:      clt,
		conn:     conn,
		isClosed: atomic.NewBool(false),
	}

	// print a error log if the client is not closed before GC
	setFinalizerClosedClient(logger, wrapClt)

	return wrapClt
}
