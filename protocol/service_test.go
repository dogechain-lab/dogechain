package protocol

import (
	"context"
	"log"
	"net"
	"testing"

	"github.com/dogechain-lab/dogechain/protocol/proto"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
)

const bufSize = 1024 * 1024

func newMockGrpcClient(t *testing.T, service *syncPeerService) proto.V1Client {
	t.Helper()

	lis := bufconn.Listen(bufSize)
	s := grpc.NewServer()
	proto.RegisterV1Server(s, service)

	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatal(err)
		}
	}()

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(
			func(ctx context.Context, address string) (net.Conn, error) {
				return lis.Dial()
			},
		),
	)

	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		defer conn.Close()
	})

	return proto.NewV1Client(conn)
}

func Test_syncPeerService_GetBlocks(t *testing.T) {
	t.Parallel()

	blocks := createMockBlocks(10)

	tests := []struct {
		name           string
		from           uint64
		to             uint64
		latest         uint64
		blocks         []*types.Block
		receivedBlocks []*types.Block
		err            error
	}{
		{
			name:           "should send the blocks to the latest",
			from:           5,
			to:             10,
			latest:         10,
			blocks:         blocks,
			receivedBlocks: blocks[4:], // from 5
			err:            nil,
		},
		{
			name:           "should return error",
			from:           5,
			to:             10,
			latest:         10,
			blocks:         blocks[:8],
			receivedBlocks: blocks[4:8], // from 5
			err:            errBlockNotFound,
		},
	}

	for _, test := range tests {
		blks := test.blocks
		from := test.from
		latest := test.latest
		receivedBlocks := test.receivedBlocks
		errVal := test.err

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			blockMap := make(map[uint64]*types.Block)

			for _, b := range blks {
				blockMap[b.Number()] = b
			}

			service := &syncPeerService{
				blockchain: &mockBlockchain{
					headerHandler: newSimpleHeaderHandler(latest),
					getBlockByNumberHandler: func(u uint64, _ bool) (*types.Block, bool) {
						block, ok := blockMap[u]
						if !ok {
							return nil, false
						}

						return block, true
					},
				},
			}

			client := newMockGrpcClient(t, service)

			rsp, err := client.GetBlocks(context.Background(), &proto.GetBlocksRequest{
				From: from,
				To:   latest,
			})

			if err == nil {
				count := 0

				for _, block := range rsp.Blocks {
					expected := receivedBlocks[count].MarshalRLP()

					assert.Equal(t, expected, block)

					count++
				}
			} else {
				assert.Contains(t, err.Error(), errVal.Error())
			}
		})
	}
}

func TestGetStatus(t *testing.T) {
	t.Parallel()

	headerNumber := uint64(10)

	service := &syncPeerService{
		blockchain: &mockBlockchain{
			headerHandler: newSimpleHeaderHandler(headerNumber),
		},
	}

	client := newMockGrpcClient(t, service)

	status, err := client.GetStatus(context.Background(), &emptypb.Empty{})

	assert.NoError(t, err)
	assert.Equal(t, headerNumber, status.Number)
}
