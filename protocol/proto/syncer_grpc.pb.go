// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package proto

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// SyncPeerClient is the client API for SyncPeer service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type SyncPeerClient interface {
	// Returns stream of blocks beginning specified from
	GetBlocks(ctx context.Context, in *GetBlocksRequest, opts ...grpc.CallOption) (SyncPeer_GetBlocksClient, error)
	// Returns server's status
	GetStatus(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*SyncPeerStatus, error)
}

type syncPeerClient struct {
	cc grpc.ClientConnInterface
}

func NewSyncPeerClient(cc grpc.ClientConnInterface) SyncPeerClient {
	return &syncPeerClient{cc}
}

func (c *syncPeerClient) GetBlocks(ctx context.Context, in *GetBlocksRequest, opts ...grpc.CallOption) (SyncPeer_GetBlocksClient, error) {
	stream, err := c.cc.NewStream(ctx, &SyncPeer_ServiceDesc.Streams[0], "/v1.SyncPeer/GetBlocks", opts...)
	if err != nil {
		return nil, err
	}
	x := &syncPeerGetBlocksClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type SyncPeer_GetBlocksClient interface {
	Recv() (*Block, error)
	grpc.ClientStream
}

type syncPeerGetBlocksClient struct {
	grpc.ClientStream
}

func (x *syncPeerGetBlocksClient) Recv() (*Block, error) {
	m := new(Block)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *syncPeerClient) GetStatus(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*SyncPeerStatus, error) {
	out := new(SyncPeerStatus)
	err := c.cc.Invoke(ctx, "/v1.SyncPeer/GetStatus", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SyncPeerServer is the server API for SyncPeer service.
// All implementations must embed UnimplementedSyncPeerServer
// for forward compatibility
type SyncPeerServer interface {
	// Returns stream of blocks beginning specified from
	GetBlocks(*GetBlocksRequest, SyncPeer_GetBlocksServer) error
	// Returns server's status
	GetStatus(context.Context, *emptypb.Empty) (*SyncPeerStatus, error)
	mustEmbedUnimplementedSyncPeerServer()
}

// UnimplementedSyncPeerServer must be embedded to have forward compatible implementations.
type UnimplementedSyncPeerServer struct {
}

func (UnimplementedSyncPeerServer) GetBlocks(*GetBlocksRequest, SyncPeer_GetBlocksServer) error {
	return status.Errorf(codes.Unimplemented, "method GetBlocks not implemented")
}
func (UnimplementedSyncPeerServer) GetStatus(context.Context, *emptypb.Empty) (*SyncPeerStatus, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetStatus not implemented")
}
func (UnimplementedSyncPeerServer) mustEmbedUnimplementedSyncPeerServer() {}

// UnsafeSyncPeerServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to SyncPeerServer will
// result in compilation errors.
type UnsafeSyncPeerServer interface {
	mustEmbedUnimplementedSyncPeerServer()
}

func RegisterSyncPeerServer(s grpc.ServiceRegistrar, srv SyncPeerServer) {
	s.RegisterService(&SyncPeer_ServiceDesc, srv)
}

func _SyncPeer_GetBlocks_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(GetBlocksRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(SyncPeerServer).GetBlocks(m, &syncPeerGetBlocksServer{stream})
}

type SyncPeer_GetBlocksServer interface {
	Send(*Block) error
	grpc.ServerStream
}

type syncPeerGetBlocksServer struct {
	grpc.ServerStream
}

func (x *syncPeerGetBlocksServer) Send(m *Block) error {
	return x.ServerStream.SendMsg(m)
}

func _SyncPeer_GetStatus_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SyncPeerServer).GetStatus(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/v1.SyncPeer/GetStatus",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SyncPeerServer).GetStatus(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

// SyncPeer_ServiceDesc is the grpc.ServiceDesc for SyncPeer service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var SyncPeer_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "v1.SyncPeer",
	HandlerType: (*SyncPeerServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetStatus",
			Handler:    _SyncPeer_GetStatus_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "GetBlocks",
			Handler:       _SyncPeer_GetBlocks_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "protocol/proto/syncer.proto",
}
