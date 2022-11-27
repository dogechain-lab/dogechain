package protocol

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/dogechain-lab/dogechain/blockchain"
	"github.com/dogechain-lab/dogechain/network"
	"github.com/dogechain-lab/dogechain/network/event"
	"github.com/dogechain-lab/dogechain/protocol/proto"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/hashicorp/go-hclog"
	"github.com/libp2p/go-libp2p-core/peer"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	SyncPeerClientLoggerName = "sync-peer-client"
	statusTopicName          = "/dogechain/syncer/status/0.1"
	defaultTimeoutForStatus  = 10 * time.Second
)

type syncPeerClient struct {
	logger     hclog.Logger // logger used for console logging
	network    Network      // reference to the network module
	blockchain Blockchain   // reference to the blockchain module

	subscription           blockchain.Subscription // reference to the blockchain subscription
	topic                  *network.Topic          // reference to the network topic
	id                     string                  // node id
	peerStatusUpdateCh     chan *NoForkPeer        // peer status update channel
	peerConnectionUpdateCh chan *event.PeerEvent   // peer connection update channel

	shouldEmitBlocks bool // flag for emitting blocks in the topic
}

func NewSyncPeerClient(
	logger hclog.Logger,
	network Network,
	blockchain Blockchain,
) SyncPeerClient {
	return &syncPeerClient{
		logger:                 logger.Named(SyncPeerClientLoggerName),
		network:                network,
		blockchain:             blockchain,
		id:                     network.AddrInfo().ID.String(),
		peerStatusUpdateCh:     make(chan *NoForkPeer, 1),
		peerConnectionUpdateCh: make(chan *event.PeerEvent, 1),
		shouldEmitBlocks:       true,
	}
}

// Start processes for SyncPeerClient
func (client *syncPeerClient) Start() error {
	go client.startNewBlockProcess()
	go client.startPeerEventProcess()

	if err := client.startGossip(); err != nil {
		return err
	}

	return nil
}

// Close terminates running processes for SyncPeerClient
func (client *syncPeerClient) Close() {
	if client.subscription != nil {
		client.subscription.Close()

		client.subscription = nil
	}

	close(client.peerStatusUpdateCh)
	close(client.peerConnectionUpdateCh)

	if client.topic != nil {
		// close topic when needed
		client.topic.Close()

		client.topic = nil
	}
}

// DisablePublishingPeerStatus disables publishing own status via gossip
func (client *syncPeerClient) DisablePublishingPeerStatus() {
	client.shouldEmitBlocks = false
}

// EnablePublishingPeerStatus enables publishing own status via gossip
func (client *syncPeerClient) EnablePublishingPeerStatus() {
	client.shouldEmitBlocks = true
}

// GetPeerStatus fetches peer status
func (client *syncPeerClient) GetPeerStatus(peerID peer.ID) (*NoForkPeer, error) {
	clt, err := client.newSyncPeerClient(peerID)
	if err != nil {
		return nil, err
	}

	timeoutCtx, cancel := context.WithTimeout(context.Background(), defaultTimeoutForStatus)
	defer cancel()

	status, err := clt.GetStatus(timeoutCtx, &emptypb.Empty{})
	if err != nil {
		return nil, err
	}

	return &NoForkPeer{
		ID:     peerID,
		Number: status.Number,
		// Distance: m.network.GetPeerDistance(peerID),
	}, nil
}

// GetConnectedPeerStatuses fetches the statuses of all connecting peers
func (client *syncPeerClient) GetConnectedPeerStatuses() []*NoForkPeer {
	var (
		ps            = client.network.Peers()
		syncPeers     = make([]*NoForkPeer, 0, len(ps))
		syncPeersLock sync.Mutex
		wg            sync.WaitGroup
	)

	for _, p := range ps {
		p := p

		wg.Add(1)

		go func() {
			defer wg.Done()

			peerID := p.Info.ID

			status, err := client.GetPeerStatus(peerID)
			if err != nil {
				client.logger.Warn("failed to get status from a peer, skip", "id", peerID, "err", err)
			}

			syncPeersLock.Lock()

			syncPeers = append(syncPeers, status)

			syncPeersLock.Unlock()
		}()
	}

	wg.Wait()

	return syncPeers
}

// GetPeerStatusUpdateCh returns a channel of peer's status update
func (client *syncPeerClient) GetPeerStatusUpdateCh() <-chan *NoForkPeer {
	return client.peerStatusUpdateCh
}

// GetPeerConnectionUpdateEventCh returns peer's connection change event
func (client *syncPeerClient) GetPeerConnectionUpdateEventCh() <-chan *event.PeerEvent {
	return client.peerConnectionUpdateCh
}

// startGossip creates new topic and starts subscribing
func (client *syncPeerClient) startGossip() error {
	topic, err := client.network.NewTopic(statusTopicName, &proto.SyncPeerStatus{})
	if err != nil {
		return err
	}

	if err := topic.Subscribe(client.handleStatusUpdate); err != nil {
		return fmt.Errorf("unable to subscribe to gossip topic, %w", err)
	}

	client.topic = topic

	return nil
}

// handleStatusUpdate is a handler of gossip
func (client *syncPeerClient) handleStatusUpdate(obj interface{}, from peer.ID) {
	client.logger.Debug("handleStatusUpdate")

	status, ok := obj.(*proto.SyncPeerStatus)
	if !ok {
		client.logger.Error("failed to cast gossiped message to txn")

		return
	}

	if !client.network.IsConnected(from) {
		if client.id != from.String() {
			client.logger.Debug("received status from non-connected peer, ignore", "id", from)
		}

		return
	}

	client.peerStatusUpdateCh <- &NoForkPeer{
		ID:     from,
		Number: status.Number,
		// Distance: m.network.GetPeerDistance(from),
	}
}

// startNewBlockProcess starts blockchain event subscription
func (client *syncPeerClient) startNewBlockProcess() {
	client.subscription = client.blockchain.SubscribeEvents()

	for event := range client.subscription.GetEventCh() {
		if !client.shouldEmitBlocks {
			continue
		}

		if l := len(event.NewChain); l > 0 {
			latest := event.NewChain[l-1]
			client.logger.Debug("client try to publish status", "latest", latest.Number)
			// Publish status
			if err := client.topic.Publish(&proto.SyncPeerStatus{
				Number: latest.Number,
			}); err != nil {
				client.logger.Warn("failed to publish status", "err", err)
			}
		}
	}
}

// startPeerEventProcess starts subscribing peer connection change events and process them
func (client *syncPeerClient) startPeerEventProcess() {
	peerEventCh, err := client.network.SubscribeCh(context.Background())
	if err != nil {
		client.logger.Error("failed to subscribe", "err", err)

		return
	}

	for e := range peerEventCh {
		if e.Type == event.PeerConnected || e.Type == event.PeerDisconnected {
			client.peerConnectionUpdateCh <- e
		}
	}
}

// CloseStream closes stream
func (client *syncPeerClient) CloseStream(peerID peer.ID) error {
	return client.network.CloseProtocolStream(_syncerV1, peerID)
}

// GetBlocks returns a stream of blocks from given height to peer's latest
func (client *syncPeerClient) GetBlocks(
	peerID peer.ID,
	from uint64,
	timeoutPerBlock time.Duration,
) (<-chan *types.Block, error) {
	clt, err := client.newSyncPeerClient(peerID)
	if err != nil {
		return nil, fmt.Errorf("failed to create sync peer client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	stream, err := clt.GetBlocks(ctx, &proto.GetBlocksRequest{
		From: from,
	})
	if err != nil {
		cancel()

		return nil, fmt.Errorf("failed to open GetBlocks stream: %w", err)
	}

	// input channel
	streamBlockCh, streamErrorCh := blockStreamToChannel(stream)

	// output channel
	blockCh := make(chan *types.Block, 1)

	go func() {
		defer func() {
			close(blockCh)
			cancel()
		}()

		for {
			select {
			case block, ok := <-streamBlockCh:
				if !ok {
					return
				}

				blockCh <- block
			case err, ok := <-streamErrorCh:
				if !ok {
					return
				}

				client.logger.Error("failed to get block from gRPC stream", "peer", peerID, "err", err)

				return
			case <-time.After(timeoutPerBlock):
				client.logger.Warn("block doesn't reach within timeout", "timeout", timeoutPerBlock)

				return
			}
		}
	}()

	return blockCh, nil
}

// newSyncPeerClient creates gRPC client
func (client *syncPeerClient) newSyncPeerClient(peerID peer.ID) (proto.V1Client, error) {
	conn, err := client.network.NewProtoConnection(_syncerV1, peerID)
	if err != nil {
		return nil, fmt.Errorf("failed to open a stream, err %w", err)
	}

	client.network.SaveProtocolStream(_syncerV1, conn, peerID)

	return proto.NewV1Client(conn), nil
}

// fromProto gets block from gRPC response data
func fromProto(protoBlock *proto.Block) (*types.Block, error) {
	block := &types.Block{}
	if err := block.UnmarshalRLP(protoBlock.Block); err != nil {
		return nil, err
	}

	return block, nil
}

func blockStreamToChannel(stream proto.V1_GetBlocksClient) (<-chan *types.Block, <-chan error) {
	blockCh := make(chan *types.Block)
	errorCh := make(chan error)

	go func() {
		defer func() {
			close(errorCh)
			close(blockCh)
		}()

		for {
			protoBlock, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}

			if err != nil {
				errorCh <- err

				break
			}

			block, err := fromProto(protoBlock)
			if err != nil {
				errorCh <- err

				break
			}

			blockCh <- block
		}
	}()

	return blockCh, errorCh
}