package protocol

import (
	"context"
	"errors"
	"fmt"

	"github.com/dogechain-lab/dogechain/network/grpc"
	"github.com/dogechain-lab/dogechain/protocol/proto"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/protobuf/types/known/anypb"
)

var (
	errBlockNotFound         = errors.New("block not found")
	errInvalidHeadersRequest = errors.New("cannot provide both a number and a hash")
)

type syncPeerService struct {
	proto.UnimplementedV1Server

	blockchain Blockchain       // blockchain service
	network    Network          // network service
	stream     *grpc.GrpcStream // grpc stream controlling

	// deprecated fields
	syncer *noForkSyncer // for rpc unary querying
}

func NewSyncPeerService(
	network Network,
	blockchain Blockchain,
) SyncPeerService {
	return &syncPeerService{
		blockchain: blockchain,
		network:    network,
	}
}

// Start starts syncPeerService
func (s *syncPeerService) Start() {
	s.setupGRPCServer()
}

// Close closes syncPeerService
func (s *syncPeerService) Close() error {
	return s.stream.Close()
}

func (s *syncPeerService) SetSyncer(syncer *noForkSyncer) {
	s.syncer = syncer
}

// setupGRPCServer setup GRPC server
func (s *syncPeerService) setupGRPCServer() {
	s.stream = grpc.NewGrpcStream()

	proto.RegisterV1Server(s.stream.GrpcServer(), s)
	s.stream.Serve()
	s.network.RegisterProtocol(_syncerV1, s.stream)
}

// GetBlocks is a gRPC endpoint to return blocks from the specific height via stream
func (s *syncPeerService) GetBlocks(
	req *proto.GetBlocksRequest,
	stream proto.V1_GetBlocksServer,
) error {
	// from to latest
	for i := req.From; i <= s.blockchain.Header().Number; i++ {
		block, ok := s.blockchain.GetBlockByNumber(i, true)
		if !ok {
			return errBlockNotFound
		}

		resp := toProtoBlock(block)

		// if client closes stream, context.Canceled is given
		if err := stream.Send(resp); err != nil {
			break
		}
	}

	return nil
}

// GetStatus is a gRPC endpoint to return the latest block number as a node status
func (s *syncPeerService) GetStatus(
	ctx context.Context,
	req *empty.Empty,
) (*proto.SyncPeerStatus, error) {
	var number uint64
	if header := s.blockchain.Header(); header != nil {
		number = header.Number
	}

	return &proto.SyncPeerStatus{
		Number: number,
	}, nil
}

// toProtoBlock converts type.Block -> proto.Block
func toProtoBlock(block *types.Block) *proto.Block {
	return &proto.Block{
		Block: block.MarshalRLP(),
	}
}

/*
 * Deprecated methods.
 */

func (s *syncPeerService) Notify(ctx context.Context, req *proto.NotifyReq) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}

// GetCurrent implements the V1Server interface
func (s *syncPeerService) GetCurrent(_ context.Context, _ *empty.Empty) (*proto.V1Status, error) {
	return s.syncer.status.toProto(), nil
}

// GetObjectsByHash implements the V1Server interface
func (s *syncPeerService) GetObjectsByHash(_ context.Context, req *proto.HashRequest) (*proto.Response, error) {
	hashes, err := req.DecodeHashes()
	if err != nil {
		return nil, err
	}

	resp := &proto.Response{
		Objs: []*proto.Response_Component{},
	}

	type rlpObject interface {
		MarshalRLPTo(dst []byte) []byte
		UnmarshalRLP(input []byte) error
	}

	for _, hash := range hashes {
		var obj rlpObject

		if req.Type == proto.HashRequest_BODIES {
			obj, _ = s.blockchain.GetBodyByHash(hash)
		} else if req.Type == proto.HashRequest_RECEIPTS {
			var raw []*types.Receipt
			raw, err = s.blockchain.GetReceiptsByHash(hash)
			if err != nil {
				return nil, err
			}

			receipts := types.Receipts(raw)
			obj = &receipts
		}

		var data []byte
		if obj != nil {
			data = obj.MarshalRLPTo(nil)
		} else {
			data = []byte{}
		}

		resp.Objs = append(resp.Objs, &proto.Response_Component{
			Spec: &anypb.Any{
				Value: data,
			},
		})
	}

	return resp, nil
}

const maxSkeletonHeadersAmount = 190

// GetHeaders implements the V1Server interface
func (s *syncPeerService) GetHeaders(_ context.Context, req *proto.GetHeadersRequest) (*proto.Response, error) {
	if req.Number != 0 && req.Hash != "" {
		return nil, errInvalidHeadersRequest
	}

	if req.Amount > maxSkeletonHeadersAmount {
		req.Amount = maxSkeletonHeadersAmount
	}

	var (
		origin *types.Header
		ok     bool
	)

	if req.Number != 0 {
		origin, ok = s.blockchain.GetHeaderByNumber(uint64(req.Number))
	} else {
		var hash types.Hash
		if err := hash.UnmarshalText([]byte(req.Hash)); err != nil {
			return nil, err
		}
		origin, ok = s.blockchain.GetHeaderByHash(hash)
	}

	if !ok {
		// return empty
		return &proto.Response{}, nil
	}

	skip := req.Skip + 1

	resp := &proto.Response{
		Objs: []*proto.Response_Component{},
	}
	addData := func(h *types.Header) {
		resp.Objs = append(resp.Objs, &proto.Response_Component{
			Spec: &anypb.Any{
				Value: h.MarshalRLPTo(nil),
			},
		})
	}

	// resp
	addData(origin)

	for count := int64(1); count < req.Amount; {
		block := int64(origin.Number) + skip

		if block < 0 {
			break
		}

		origin, ok = s.blockchain.GetHeaderByNumber(uint64(block))

		if !ok {
			break
		}
		count++

		// resp
		addData(origin)
	}

	return resp, nil
}

// Helper functions to decode responses from the grpc layer
func getBodies(ctx context.Context, clt proto.V1Client, hashes []types.Hash) ([]*types.Body, error) {
	input := make([]string, 0, len(hashes))

	for _, h := range hashes {
		input = append(input, h.String())
	}

	resp, err := clt.GetObjectsByHash(
		ctx,
		&proto.HashRequest{
			Hash: input,
			Type: proto.HashRequest_BODIES,
		},
	)
	if err != nil {
		return nil, err
	}

	res := make([]*types.Body, 0, len(resp.Objs))

	for _, obj := range resp.Objs {
		var body types.Body
		if obj.Spec.Value != nil {
			if err := body.UnmarshalRLP(obj.Spec.Value); err != nil {
				return nil, err
			}
		}

		res = append(res, &body)
	}

	if len(res) != len(input) {
		return nil, fmt.Errorf("not correct size")
	}

	return res, nil
}
