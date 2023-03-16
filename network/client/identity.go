//nolint:dupl
package client

import (
	"context"

	"github.com/dogechain-lab/dogechain/network/proto"
	"github.com/hashicorp/go-hclog"
	"go.uber.org/atomic"

	rawGrpc "google.golang.org/grpc"
)

type IdentityClient interface {
	GrpcClientCloser

	Hello(ctx context.Context, in *proto.Status) (*proto.Status, error)
}

type identityClient struct {
	clt  proto.IdentityClient
	conn *rawGrpc.ClientConn

	isClosed *atomic.Bool

	metrics Metrics
}

const (
	/**
	from grpc.UnaryServerInfo

	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/v1.Identity/Hello",
	}
	**/
	methodNameHello = "/v1.Identity/Hello"
)

func (i *identityClient) Hello(ctx context.Context, in *proto.Status) (*proto.Status, error) {
	i.metrics.rpcMethodCallCountInc(methodNameHello)

	begin := i.metrics.rpcMethodCallBegin(methodNameHello)
	defer i.metrics.rpcMethodCallEnd(methodNameHello, begin)

	status, err := i.clt.Hello(ctx, in, rawGrpc.WaitForReady(false))
	if err != nil {
		i.metrics.rpcMethodCallErrorCountInc(methodNameHello)

		return nil, err
	}

	return status, nil
}

func (i *identityClient) Close() error {
	if i.isClosed.CompareAndSwap(false, true) {
		return i.conn.Close()
	} else {
		return ErrClientClosed
	}
}

func (i *identityClient) IsClose() bool {
	return i.isClosed.Load()
}

func NewIdentityClient(
	logger hclog.Logger,
	metrics Metrics,
	clt proto.IdentityClient,
	conn *rawGrpc.ClientConn,
) IdentityClient {
	wrapClt := &identityClient{
		clt:      clt,
		conn:     conn,
		isClosed: atomic.NewBool(false),
		metrics:  metrics,
	}

	// print a error log if the client is not closed before GC
	setFinalizerClosedClient(logger, wrapClt)

	return wrapClt
}
