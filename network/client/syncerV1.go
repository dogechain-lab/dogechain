package client

import (
	"context"

	"github.com/dogechain-lab/dogechain/protocol/proto"
	"github.com/hashicorp/go-hclog"
	"go.uber.org/atomic"
	rawGrpc "google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type SyncerV1Client interface {
	GrpcClientCloser

	GetCurrent(ctx context.Context, in *emptypb.Empty) (*proto.V1Status, error)
	GetObjectsByHash(ctx context.Context, in *proto.HashRequest) (*proto.Response, error)
	GetHeaders(ctx context.Context, in *proto.GetHeadersRequest) (*proto.Response, error)

	Notify(ctx context.Context, in *proto.NotifyReq) (*emptypb.Empty, error)
	// Returns blocks from begin to end (which is determined by server)
	GetBlocks(ctx context.Context, in *proto.GetBlocksRequest) (*proto.GetBlocksResponse, error)
	// Returns server's status
	GetStatus(ctx context.Context, in *emptypb.Empty) (*proto.SyncPeerStatus, error)
}

type syncerV1Client struct {
	clt  proto.V1Client
	conn *rawGrpc.ClientConn

	isClosed *atomic.Bool
}

func (i *syncerV1Client) GetCurrent(ctx context.Context, in *emptypb.Empty) (*proto.V1Status, error) {
	return i.clt.GetCurrent(ctx, in, rawGrpc.WaitForReady(false))
}

func (i *syncerV1Client) GetObjectsByHash(ctx context.Context, in *proto.HashRequest) (*proto.Response, error) {
	return i.clt.GetObjectsByHash(ctx, in, rawGrpc.WaitForReady(false))
}

func (i *syncerV1Client) GetHeaders(ctx context.Context, in *proto.GetHeadersRequest) (*proto.Response, error) {
	return i.clt.GetHeaders(ctx, in, rawGrpc.WaitForReady(false))
}

func (i *syncerV1Client) Notify(ctx context.Context, in *proto.NotifyReq) (*emptypb.Empty, error) {
	return i.clt.Notify(ctx, in, rawGrpc.WaitForReady(false))
}

func (i *syncerV1Client) GetBlocks(ctx context.Context, in *proto.GetBlocksRequest) (*proto.GetBlocksResponse, error) {
	return i.clt.GetBlocks(ctx, in, rawGrpc.WaitForReady(false))
}

func (i *syncerV1Client) GetStatus(ctx context.Context, in *emptypb.Empty) (*proto.SyncPeerStatus, error) {
	return i.clt.GetStatus(ctx, in, rawGrpc.WaitForReady(false))
}

func (i *syncerV1Client) Close() error {
	if i.isClosed.CompareAndSwap(false, true) {
		return i.conn.Close()
	} else {
		return ErrClientClosed
	}
}

func (i *syncerV1Client) IsClose() bool {
	return i.isClosed.Load()
}

func NewSyncerV1Client(
	logger hclog.Logger,
	clt proto.V1Client,
	conn *rawGrpc.ClientConn,
) SyncerV1Client {
	wrapClt := &syncerV1Client{
		clt:      clt,
		conn:     conn,
		isClosed: atomic.NewBool(false),
	}

	// print a error log if the client is not closed before GC
	setFinalizerClosedClient(logger, wrapClt)

	return wrapClt
}
