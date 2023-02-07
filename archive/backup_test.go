package archive

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/dogechain-lab/dogechain/server/proto"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	genesis = &types.Block{
		Header: &types.Header{
			Hash:   types.StringToHash("genesis"),
			Number: 0,
		},
	}
	blocks = []*types.Block{
		genesis,
		{
			Header: &types.Header{
				Number: 1,
			},
		},
		{
			Header: &types.Header{
				Number: 2,
			},
		},
		{
			Header: &types.Header{
				Number: 3,
			},
		},
	}
)

func init() {
	genesis.Header.ComputeHash()

	for _, b := range blocks {
		b.Header.ComputeHash()
	}
}

type systemClientMock struct {
	proto.SystemClient
	status       *proto.ServerStatus
	errForStatus error
	blocks       []*types.Block
	errForBlock  error
}

func (m *systemClientMock) GetStatus(context.Context, *emptypb.Empty, ...grpc.CallOption) (*proto.ServerStatus, error) {
	return m.status, m.errForStatus
}

func (m *systemClientMock) BlockByNumber(
	_ctx context.Context,
	req *proto.BlockByNumberRequest,
	_opts ...grpc.CallOption,
) (*proto.BlockResponse, error) {
	// find block by request number
	for _, b := range m.blocks {
		if b.Header.Number == req.Number {
			return &proto.BlockResponse{
				Data: b.MarshalRLP(),
			}, m.errForBlock
		}
	}

	return nil, m.errForBlock
}

func Test_determineTo(t *testing.T) {
	toPtr := func(x uint64) *uint64 {
		return &x
	}

	tests := []struct {
		name string
		// input
		targetTo *uint64
		// mock
		systemClientMock proto.SystemClient
		// result
		resTo     uint64
		resToHash types.Hash
		err       error
	}{
		{
			name:     "should return expected 'to'",
			targetTo: toPtr(blocks[2].Number()),
			systemClientMock: &systemClientMock{
				status: &proto.ServerStatus{
					Current: &proto.ServerStatus_Block{
						// greater than targetTo
						Number: 10,
					},
				},
				blocks: blocks,
			},
			resTo:     blocks[2].Number(),
			resToHash: blocks[2].Hash(),
			err:       nil,
		},
		{
			name:     "should return latest if target to is greater than the latest in node",
			targetTo: toPtr(blocks[2].Number()),
			systemClientMock: &systemClientMock{
				status: &proto.ServerStatus{
					Current: &proto.ServerStatus_Block{
						// less than targetTo
						Number: int64(blocks[1].Number()),
						Hash:   blocks[1].Hash().String(),
					},
				},
				blocks: blocks,
			},
			resTo:     blocks[1].Number(),
			resToHash: blocks[1].Hash(),
			err:       nil,
		},
		{
			name:     "should fail if GetStatus failed",
			targetTo: toPtr(2),
			systemClientMock: &systemClientMock{
				status:       nil,
				errForStatus: errors.New("fake error"),
			},
			resTo:     0,
			resToHash: types.Hash{},
			err:       errors.New("fake error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resTo, resToHash, err := determineTo(context.Background(), tt.systemClientMock, tt.targetTo)
			assert.Equal(t, tt.err, err)
			if tt.err == nil {
				assert.Equal(t, tt.resTo, resTo)
				assert.Equal(t, tt.resToHash, resToHash)
			}
		})
	}
}

func Test_processExportStream(t *testing.T) {
	tests := []struct {
		name             string
		systemClientMock proto.SystemClient
		// result
		from  uint64
		to    uint64
		err   error
		recvs []*proto.BlockResponse
	}{
		{
			name: "should be succeed with single block",
			systemClientMock: &systemClientMock{
				status: &proto.ServerStatus{
					Current: &proto.ServerStatus_Block{
						// greater than targetTo
						Number: int64(blocks[2].Number()),
					},
				},
				blocks: blocks,
			},
			from: blocks[1].Number(),
			to:   blocks[1].Number(),
			err:  nil,
			recvs: []*proto.BlockResponse{
				{
					Data: blocks[1].MarshalRLP(),
				},
			},
		},
		{
			name: "should be succeed multiple blocks",
			systemClientMock: &systemClientMock{
				status: &proto.ServerStatus{
					Current: &proto.ServerStatus_Block{
						// greater than targetTo
						Number: int64(blocks[2].Number()),
					},
				},
				blocks: blocks,
			},
			from: blocks[0].Number(),
			to:   blocks[2].Number(),
			err:  nil,
			recvs: []*proto.BlockResponse{
				{
					Data: blocks[0].MarshalRLP(),
				},
				{
					Data: blocks[1].MarshalRLP(),
				},
				{
					Data: blocks[2].MarshalRLP(),
				},
			},
		},
		{
			name: "should be succeed select range of 2-3 blocks",
			systemClientMock: &systemClientMock{
				status: &proto.ServerStatus{
					Current: &proto.ServerStatus_Block{
						// greater than targetTo
						Number: int64(blocks[2].Number()),
					},
				},
				blocks: blocks,
			},
			from: blocks[1].Number(),
			to:   blocks[2].Number(),
			err:  nil,
			recvs: []*proto.BlockResponse{
				{
					Data: blocks[1].MarshalRLP(),
				},
				{
					Data: blocks[2].MarshalRLP(),
				},
			},
		},
		{
			name: "should be failed with error range",
			systemClientMock: &systemClientMock{
				status: &proto.ServerStatus{
					Current: &proto.ServerStatus_Block{
						// greater than targetTo
						Number: int64(blocks[2].Number()),
					},
				},
				blocks: blocks,
			},
			from:  blocks[2].Number(),
			to:    blocks[1].Number(),
			err:   ErrBlockRange,
			recvs: []*proto.BlockResponse{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buffer bytes.Buffer
			var ctx, cancel = context.WithCancel(context.Background())
			defer cancel()

			from, to, err := processExport(ctx, tt.systemClientMock, hclog.NewNullLogger(), &buffer, tt.from, tt.to)

			assert.Equal(t, tt.err, err)
			if err != nil {
				return
			}

			assert.Equal(t, tt.from, from)
			assert.Equal(t, tt.to, to)

			// create expected data
			expectedData := make([]byte, 0)
			for _, rv := range tt.recvs {
				if rv != nil {
					expectedData = append(expectedData, rv.Data...)
				}
			}
			assert.Equal(t, expectedData, buffer.Bytes())
		})
	}
}
