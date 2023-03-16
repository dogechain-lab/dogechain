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

	metrics Metrics
}

const (
	/**
	from grpc.UnaryServerInfo

	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/v1.Syncer/GetCurrent",
	}
	**/

	methodNameGetCurrent       = "/v1.Syncer/GetCurrent"
	methodNameGetObjectsByHash = "/v1.Syncer/GetObjectsByHash"
	methodNameGetHeaders       = "/v1.Syncer/GetHeaders"
	methodNameNotify           = "/v1.Syncer/Notify"
	methodNameGetBlocks        = "/v1.Syncer/GetBlocks"
	methodNameGetStatus        = "/v1.Syncer/GetStatus"
)

func (i *syncerV1Client) wrapMetricsCall(name methodName, call func() error) {
	i.metrics.rpcMethodCallCountInc(name)

	begin := i.metrics.rpcMethodCallBegin(name)
	defer i.metrics.rpcMethodCallEnd(name, begin)

	err := call()
	if err != nil {
		i.metrics.rpcMethodCallErrorCountInc(name)
	}
}

func (i *syncerV1Client) GetCurrent(
	ctx context.Context,
	in *emptypb.Empty,
) (status *proto.V1Status, err error) {
	i.wrapMetricsCall(methodNameGetCurrent, func() error {
		status, err = i.clt.GetCurrent(ctx, in, rawGrpc.WaitForReady(false))

		return err
	})

	return status, err
}

func (i *syncerV1Client) GetObjectsByHash(
	ctx context.Context,
	in *proto.HashRequest,
) (response *proto.Response, err error) {
	i.wrapMetricsCall(methodNameGetObjectsByHash, func() error {
		response, err = i.clt.GetObjectsByHash(
			ctx,
			in,
			rawGrpc.WaitForReady(false),
		)

		return err
	})

	return response, err
}

func (i *syncerV1Client) GetHeaders(
	ctx context.Context,
	in *proto.GetHeadersRequest,
) (response *proto.Response, err error) {
	i.wrapMetricsCall(methodNameGetHeaders, func() error {
		response, err = i.clt.GetHeaders(ctx, in, rawGrpc.WaitForReady(false))

		return err
	})

	return response, err
}

func (i *syncerV1Client) Notify(ctx context.Context, in *proto.NotifyReq) (response *emptypb.Empty, err error) {
	i.wrapMetricsCall(methodNameNotify, func() error {
		response, err = i.clt.Notify(ctx, in, rawGrpc.WaitForReady(false))

		return err
	})

	return response, err
}

func (i *syncerV1Client) GetBlocks(
	ctx context.Context,
	in *proto.GetBlocksRequest,
) (response *proto.GetBlocksResponse, err error) {
	i.wrapMetricsCall(methodNameGetBlocks, func() error {
		response, err = i.clt.GetBlocks(ctx, in, rawGrpc.WaitForReady(false))

		return err
	})

	return response, err
}

func (i *syncerV1Client) GetStatus(ctx context.Context, in *emptypb.Empty) (response *proto.SyncPeerStatus, err error) {
	i.wrapMetricsCall(methodNameGetStatus, func() error {
		response, err = i.clt.GetStatus(ctx, in, rawGrpc.WaitForReady(false))

		return err
	})

	return response, err
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
	metrics Metrics,
	clt proto.V1Client,
	conn *rawGrpc.ClientConn,
) SyncerV1Client {
	wrapClt := &syncerV1Client{
		clt:      clt,
		conn:     conn,
		isClosed: atomic.NewBool(false),
		metrics:  metrics,
	}

	// print a error log if the client is not closed before GC
	setFinalizerClosedClient(logger, wrapClt)

	return wrapClt
}
