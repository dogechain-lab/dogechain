package wrappers

import (
	"context"

	"github.com/dogechain-lab/dogechain/network/proto"

	rawGrpc "google.golang.org/grpc"
)

type IdentityClient interface {
	Hello(ctx context.Context, in *proto.Status) (*proto.Status, error)
	Close() error
}

type identityClient struct {
	clt  proto.IdentityClient
	conn *rawGrpc.ClientConn
}

func (i *identityClient) Hello(ctx context.Context, in *proto.Status) (*proto.Status, error) {
	return i.clt.Hello(ctx, in)
}

func (i *identityClient) Close() error {
	return i.conn.Close()
}

func NewIdentityClient(
	clt proto.IdentityClient,
	conn *rawGrpc.ClientConn,
) IdentityClient {
	return &identityClient{clt, conn}
}
