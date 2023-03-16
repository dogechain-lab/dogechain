package protocol

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dogechain-lab/dogechain/network"
	"github.com/dogechain-lab/dogechain/network/event"
	"github.com/dogechain-lab/dogechain/protocol/proto"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/hashicorp/go-hclog"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/atomic"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/emptypb"

	rpcClient "github.com/dogechain-lab/dogechain/network/client"
)

const (
	SyncPeerClientLoggerName = "sync-peer-client"
	statusTopicName          = "/dogechain/syncer/status/0.1"
	defaultTimeoutForStatus  = 10 * time.Second
	defaultTimeoutForBlocks  = 30 * time.Second
)

type syncPeerClient struct {
	logger hclog.Logger // logger used for console logging

	network    network.Network // reference to the network module
	blockchain Blockchain      // reference to the blockchain module

	topic                  network.Topic         // reference to the network topic
	selfID                 string                // self node id
	peerStatusUpdateCh     chan *NoForkPeer      // peer status update channel
	peerConnectionUpdateCh chan *event.PeerEvent // peer connection update channel

	shouldEmitBlocks bool // flag for emitting blocks in the topic

	isClosed *atomic.Bool

	ctx    context.Context
	cancel context.CancelFunc
}

func NewSyncPeerClient(
	logger hclog.Logger,
	network network.Network,
	blockchain Blockchain,
) SyncPeerClient {
	ctx, cancel := context.WithCancel(context.Background())

	return &syncPeerClient{
		logger:                 logger.Named(SyncPeerClientLoggerName),
		network:                network,
		blockchain:             blockchain,
		selfID:                 network.AddrInfo().ID.String(),
		peerStatusUpdateCh:     make(chan *NoForkPeer, 32),
		peerConnectionUpdateCh: make(chan *event.PeerEvent, 32),
		shouldEmitBlocks:       true,
		isClosed:               atomic.NewBool(false),
		ctx:                    ctx,
		cancel:                 cancel,
	}
}

// Start processes for SyncPeerClient
func (client *syncPeerClient) Start() error {
	go client.startNewBlockProcess()

	if err := client.subscribeEventProcess(); err != nil {
		return err
	}

	if err := client.startGossip(); err != nil {
		return err
	}

	return nil
}

// Close terminates running processes for SyncPeerClient
func (client *syncPeerClient) Close() {
	if !client.isClosed.CompareAndSwap(false, true) {
		return
	}

	client.cancel()

	if client.topic != nil {
		// close topic when needed
		client.topic.Close()
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
	timeoutCtx, cancel := context.WithTimeout(context.Background(), defaultTimeoutForStatus)
	defer cancel()

	clt, err := client.newSyncPeerClient(timeoutCtx, peerID)
	if err != nil {
		return nil, err
	}

	defer clt.Close()

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
			if peerID.String() == client.selfID {
				return
			}

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
	status, ok := obj.(*proto.SyncPeerStatus)
	if !ok {
		client.logger.Error("failed to cast gossiped message to txn")

		return
	}

	if !client.network.HasPeer(from) {
		return
	}

	client.logger.Debug("get connected peer status update", "from", from, "status", status.Number)

	if client.isClosed.Load() {
		client.logger.Debug("client is closed, ignore status update", "from", from, "status", status.Number)

		return
	}

	client.logger.Debug("send peerStatusUpdateCh")

	peer := &NoForkPeer{
		ID:     from,
		Number: status.Number,
	}

	select {
	case <-client.ctx.Done():
		return
	case client.peerStatusUpdateCh <- peer:
	default:
	}
}

// startNewBlockProcess starts blockchain event subscription
func (client *syncPeerClient) startNewBlockProcess() {
	subscription := client.blockchain.SubscribeEvents()
	defer subscription.Unsubscribe()

	for {
		if client.isClosed.Load() || subscription.IsClosed() {
			return
		}

		event, ok := <-subscription.GetEvent()
		if event == nil || !ok {
			client.logger.Debug("event is nil, skip")

			continue
		}

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

// subscribeEventProcess starts subscribing peer connection change events and process them
func (client *syncPeerClient) subscribeEventProcess() error {
	err := client.network.SubscribeFn(client.ctx, func(e *event.PeerEvent) {
		if client.isClosed.Load() {
			client.logger.Debug("client is closed, ignore peer connection update", "peer", e.PeerID, "type", e.Type)

			return
		}

		if e.Type == event.PeerConnected || e.Type == event.PeerDisconnected {
			select {
			case <-client.ctx.Done():
				return
			case client.peerConnectionUpdateCh <- e:
			}
		}
	})

	return err
}

// GetBlocks returns a stream of blocks from given height to peer's latest
func (client *syncPeerClient) GetBlocks(
	ctx context.Context,
	peerID peer.ID,
	from uint64,
	to uint64,
) ([]*types.Block, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeoutForBlocks)
	defer cancel()

	clt, err := client.newSyncPeerClient(ctx, peerID)
	if err != nil {
		return nil, fmt.Errorf("failed to create sync peer client: %w", err)
	}

	defer clt.Close()

	rsp, err := clt.GetBlocks(ctx, &proto.GetBlocksRequest{
		From: from,
		To:   to,
	})
	if err != nil {
		return nil, err
	}

	blocks := make([]*types.Block, len(rsp.Blocks))

	for i, b := range rsp.Blocks {
		block := new(types.Block)

		if err := block.UnmarshalRLP(b); err != nil {
			return nil, fmt.Errorf("failed to UnmarshalRLP: %w", err)
		}

		blocks[i] = block
	}

	return blocks, err
}

// GetConnectedPeerStatuses fetches the statuses of all connecting peers
func (client *syncPeerClient) Broadcast(block *types.Block) error {
	var ps = client.network.Peers()

	// Get the chain difficulty associated with block
	td, ok := client.blockchain.GetTD(block.Hash())
	if !ok {
		// not supposed to happen
		client.logger.Error("total difficulty not found", "block number", block.Number())

		return errBlockNotFound
	}

	// broadcast the new block to all the peers
	req := &proto.NotifyReq{
		Status: &proto.V1Status{
			Hash:       block.Hash().String(),
			Number:     block.Number(),
			Difficulty: td.String(),
		},
		Raw: &anypb.Any{
			Value: block.MarshalRLP(),
		},
	}

	for _, p := range ps {
		go func(p *network.PeerConnInfo, req *proto.NotifyReq) {
			begin := time.Now()

			if err := client.broadcastBlockTo(p.Info.ID, req); err != nil {
				client.logger.Warn("failed to broadcast block to peer", "id", p.Info.ID, "err", err)
			} else {
				client.logger.Debug("notify block to peer", "id", p.Info.ID, "duration", time.Since(begin).Seconds())
			}
		}(p, req)
	}

	return nil
}

// broadcastBlockTo sends block to peer
func (client *syncPeerClient) broadcastBlockTo(
	peerID peer.ID,
	req *proto.NotifyReq,
) error {
	// The duration is not easy to evaluate, so don't count it
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutForStatus)
	defer cancel()

	clt, err := client.newSyncPeerClient(ctx, peerID)
	if err != nil {
		return fmt.Errorf("failed to create sync peer client: %w", err)
	}

	defer clt.Close()

	_, err = clt.Notify(ctx, req)

	return err
}

// newSyncPeerClient creates gRPC client [thread safe]
func (client *syncPeerClient) newSyncPeerClient(ctx context.Context, peerID peer.ID) (rpcClient.SyncerV1Client, error) {
	// create new connection
	conn, err := client.network.NewProtoConnection(ctx, _syncerV1, peerID)
	if err != nil {
		client.network.ForgetPeer(peerID, "not support syncer v1 protocol")

		return nil, fmt.Errorf("failed to open a stream, err %w", err)
	}

	// save protocol stream
	clt := rpcClient.NewSyncerV1Client(
		client.logger,
		client.network.GetMetrics().GetGrpcMetrics(),
		proto.NewV1Client(conn),
		conn,
	)

	return clt, nil
}
