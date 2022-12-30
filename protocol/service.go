package protocol

import (
	"context"
	"errors"
	"fmt"

	"github.com/dogechain-lab/dogechain/network"
	"github.com/dogechain-lab/dogechain/network/grpc"
	"github.com/dogechain-lab/dogechain/protocol/proto"
	"github.com/dogechain-lab/dogechain/types"

	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	errBlockNotFound         = errors.New("block not found")
	errInvalidHeadersRequest = errors.New("cannot provide both a number and a hash")
	errInvalidRange          = errors.New("from to range not valid")
)

type syncPeerService struct {
	proto.UnimplementedV1Server

	blockchain Blockchain       // blockchain service
	network    network.Network  // network service
	stream     *grpc.GrpcStream // grpc stream controlling

	// deprecated fields
	syncer *noForkSyncer // for rpc unary querying
}

func NewSyncPeerService(
	network network.Network,
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

const (
	// _minCompressSize = 4 * 1024        // 4k
	_maxSendingSize = 8 * 1024 * 1024 // = 8M => 2-4M after compression, which is reasonable
)

// GetBlocks is a gRPC endpoint to return blocks from the specific height
//
// It is designed forward and backward competible.
// With accpetAlgos, client and server side can iterate their own side
func (s *syncPeerService) GetBlocks(
	ctx context.Context,
	req *proto.GetBlocksRequest,
) (*proto.GetBlocksResponse, error) {
	if req.From > req.To {
		return nil, errInvalidRange
	}

	var (
		rsp = &proto.GetBlocksResponse{
			From: req.From,
		}
		blocks = make([][]byte, 0, req.To-req.From+1)
		size   = 0
	)

	for to := req.From; to <= req.To; to++ {
		block, ok := s.blockchain.GetBlockByNumber(to, true)
		if !ok {
			return nil, errBlockNotFound
		}

		// rlp marshal
		b := block.MarshalRLP()

		// check whether compress
		size += len(b)
		if size > _maxSendingSize {
			// no more data
			break
		}

		// update response
		blocks = append(blocks, b)
		rsp.To = to
	}

	rsp.Blocks = blocks

	return rsp, nil
}

// GetStatus is a gRPC endpoint to return the latest block number as a node status
func (s *syncPeerService) GetStatus(
	ctx context.Context,
	req *emptypb.Empty,
) (*proto.SyncPeerStatus, error) {
	var number uint64
	if header := s.blockchain.Header(); header != nil {
		number = header.Number
	}

	return &proto.SyncPeerStatus{
		Number: number,
	}, nil
}

/*
 * Deprecated methods.
 */

func (s *syncPeerService) Notify(ctx context.Context, req *proto.NotifyReq) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

// GetCurrent implements the V1Server interface
func (s *syncPeerService) GetCurrent(_ context.Context, _ *emptypb.Empty) (*proto.V1Status, error) {
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
