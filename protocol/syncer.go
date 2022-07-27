package protocol

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/dogechain-lab/dogechain/blockchain"
	"github.com/dogechain-lab/dogechain/helper/progress"
	"github.com/dogechain-lab/dogechain/network"
	"github.com/dogechain-lab/dogechain/network/event"
	libp2pGrpc "github.com/dogechain-lab/dogechain/network/grpc"
	"github.com/dogechain-lab/dogechain/protocol/proto"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/hashicorp/go-hclog"
	"github.com/libp2p/go-libp2p-core/peer"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	anypb "google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	maxEnqueueSize = 50
	popTimeout     = 10 * time.Second
)

var (
	ErrLoadLocalGenesisFailed = errors.New("failed to read local genesis")
	ErrMismatchGenesis        = errors.New("genesis does not match")
	ErrCommonAncestorNotFound = errors.New("header is nil")
	ErrForkNotFound           = errors.New("fork not found")
	ErrPopTimeout             = errors.New("timeout")
	ErrConnectionClosed       = errors.New("connection closed")
	ErrTooManyHeaders         = errors.New("unexpected more than 1 result")
	ErrDecodeDifficulty       = errors.New("failed to decode difficulty")
	ErrInvalidTypeAssertion   = errors.New("invalid type assertion")
)

// blocks sorted by number (ascending)
type minNumBlockQueue []*types.Block

// must implement sort interface
var _ sort.Interface = (*minNumBlockQueue)(nil)

func (q *minNumBlockQueue) Len() int {
	return len(*q)
}

func (q *minNumBlockQueue) Less(i, j int) bool {
	return (*q)[i].Number() < (*q)[j].Number()
}

func (q *minNumBlockQueue) Swap(i, j int) {
	(*q)[i], (*q)[j] = (*q)[j], (*q)[i]
}

// Syncer is a sync protocol
type Syncer struct {
	logger     hclog.Logger
	blockchain blockchainShim

	peers sync.Map // Maps peer.ID -> SyncPeer

	serviceV1 *serviceV1
	stopCh    chan struct{}

	status     *Status
	statusLock sync.Mutex

	server *network.Server

	syncProgression *progress.ProgressionWrapper
}

// NewSyncer creates a new Syncer instance
func NewSyncer(logger hclog.Logger, server *network.Server, blockchain blockchainShim) *Syncer {
	s := &Syncer{
		logger:          logger.Named("syncer"),
		stopCh:          make(chan struct{}),
		blockchain:      blockchain,
		server:          server,
		syncProgression: progress.NewProgressionWrapper(progress.ChainSyncBulk),
	}

	return s
}

// GetSyncProgression returns the latest sync progression, if any
func (s *Syncer) GetSyncProgression() *progress.Progression {
	return s.syncProgression.GetProgression()
}

// syncCurrentStatus taps into the blockchain event steam and updates the Syncer.status field
func (s *Syncer) syncCurrentStatus() {
	sub := s.blockchain.SubscribeEvents()
	eventCh := sub.GetEventCh()

	// watch the subscription and notify
	for {
		select {
		case evnt := <-eventCh:
			if evnt.Type == blockchain.EventFork {
				// we do not want to notify forks
				continue
			}

			if len(evnt.NewChain) == 0 {
				// this should not happen
				continue
			}

			status := &Status{
				Difficulty: evnt.Difficulty,
				Hash:       evnt.NewChain[0].Hash,
				Number:     evnt.NewChain[0].Number,
			}

			s.updateStatus(status)

		case <-s.stopCh:
			sub.Close()

			return
		}
	}
}

func (s *Syncer) updateStatus(status *Status) {
	s.statusLock.Lock()
	defer s.statusLock.Unlock()

	// compare current status, would only update until new height meet or fork happens
	switch {
	case status.Number < s.status.Number:
		return
	case status.Number == s.status.Number:
		if status.Hash == s.status.Hash {
			return
		}
	}

	s.logger.Debug("update syncer status", "status", status)

	s.status = status
}

const syncerV1 = "/syncer/0.1"

// enqueueBlock adds the specific block to the peerID queue
func (s *Syncer) enqueueBlock(peerID peer.ID, b *types.Block) {
	s.logger.Debug("enqueue block", "peer", peerID, "number", b.Number(), "hash", b.Hash())

	peer, exists := s.peers.Load(peerID)
	if !exists {
		s.logger.Error("enqueue block: peer not present", "id", peerID.String())

		return
	}

	syncPeer, ok := peer.(*SyncPeer)
	if !ok {
		s.logger.Error("invalid sync peer type cast")

		return
	}

	syncPeer.appendBlock(b)
}

func (s *Syncer) updatePeerStatus(peerID peer.ID, status *Status) {
	s.logger.Debug(
		"update peer status",
		"peer",
		peerID,
		"latest block number",
		status.Number,
		"latest block hash",
		status.Hash, "difficulty",
		status.Difficulty,
	)

	if peer, ok := s.peers.Load(peerID); ok {
		syncPeer, ok := peer.(*SyncPeer)
		if !ok {
			s.logger.Error("invalid sync peer type cast")

			return
		}

		syncPeer.updateStatus(status)
	}
}

// Broadcast broadcasts a block to all peers
func (s *Syncer) Broadcast(b *types.Block) {
	sendNotify := func(peerID, peer interface{}, req *proto.NotifyReq) {
		startTime := time.Now()

		if _, err := peer.(*SyncPeer).client.Notify(context.Background(), req); err != nil {
			s.logger.Error("failed to notify", "err", err)

			return
		}

		duration := time.Since(startTime)

		s.logger.Debug(
			"notifying peer",
			"id", peerID,
			"duration", duration.Seconds(),
		)
	}

	// Get the chain difficulty associated with block
	td, ok := s.blockchain.GetTD(b.Hash())
	if !ok {
		// not supposed to happen
		s.logger.Error("total difficulty not found", "block number", b.Number())

		return
	}

	// broadcast the new block to all the peers
	req := &proto.NotifyReq{
		Status: &proto.V1Status{
			Hash:       b.Hash().String(),
			Number:     b.Number(),
			Difficulty: td.String(),
		},
		Raw: &anypb.Any{
			Value: b.MarshalRLP(),
		},
	}

	s.logger.Debug("broadcast start")
	s.peers.Range(func(peerID, peer interface{}) bool {
		go sendNotify(peerID, peer, req)

		return true
	})
	s.logger.Debug("broadcast end")
}

// Start starts the syncer protocol
func (s *Syncer) Start() {
	s.serviceV1 = &serviceV1{
		syncer: s,
		logger: s.logger.With("name", "serviceV1"),
		store:  s.blockchain,
	}

	// Get the current status of the syncer
	currentHeader := s.blockchain.Header()
	diff, _ := s.blockchain.GetTD(currentHeader.Hash)

	s.status = &Status{
		Hash:       currentHeader.Hash,
		Number:     currentHeader.Number,
		Difficulty: diff,
	}

	// Run the blockchain event listener loop
	go s.syncCurrentStatus()

	// Register the grpc protocol for syncer
	grpcStream := libp2pGrpc.NewGrpcStream()
	proto.RegisterV1Server(grpcStream.GrpcServer(), s.serviceV1)
	grpcStream.Serve()
	s.server.RegisterProtocol(syncerV1, grpcStream)

	s.setupPeers()

	go s.handlePeerEvent()
}

// setupPeers adds connected peers as syncer peers
func (s *Syncer) setupPeers() {
	for _, p := range s.server.Peers() {
		if addErr := s.AddPeer(p.Info.ID); addErr != nil {
			s.logger.Error(fmt.Sprintf("Error when adding peer [%s], %v", p.Info.ID, addErr))
		}
	}
}

// handlePeerEvent subscribes network event and adds/deletes peer from syncer
func (s *Syncer) handlePeerEvent() {
	updateCh, err := s.server.SubscribeCh()
	if err != nil {
		s.logger.Error("failed to subscribe", "err", err)

		return
	}

	go func() {
		for {
			evnt, ok := <-updateCh
			if !ok {
				return
			}

			switch evnt.Type {
			case event.PeerConnected:
				if err := s.AddPeer(evnt.PeerID); err != nil {
					s.logger.Error("failed to add peer", "err", err)
				}
			case event.PeerDisconnected:
				if err := s.DeletePeer(evnt.PeerID); err != nil {
					s.logger.Error("failed to delete user", "err", err)
				}
			}
		}
	}()
}

// BestPeer returns the best peer by difficulty (if any)
func (s *Syncer) BestPeer() *SyncPeer {
	var (
		bestPeer        *SyncPeer
		bestBlockNumber uint64
	)

	s.peers.Range(func(peerID, peer interface{}) bool {
		syncPeer, ok := peer.(*SyncPeer)
		if !ok {
			return false
		}

		peerBlockNumber := syncPeer.Number()
		if bestPeer == nil || peerBlockNumber > bestBlockNumber {
			bestPeer = syncPeer
			bestBlockNumber = peerBlockNumber
		}

		return true
	})

	if bestBlockNumber <= s.blockchain.Header().Number {
		bestPeer = nil
	}

	return bestPeer
}

// AddPeer establishes new connection with the given peer
func (s *Syncer) AddPeer(peerID peer.ID) error {
	if _, ok := s.peers.Load(peerID); ok {
		// already connected
		return nil
	}

	stream, err := s.server.NewStream(syncerV1, peerID)
	if err != nil {
		return fmt.Errorf("failed to open a stream, err %w", err)
	}

	conn := libp2pGrpc.WrapClient(stream)

	// watch for changes of the other node first
	clt := proto.NewV1Client(conn)

	rawStatus, err := clt.GetCurrent(context.Background(), &emptypb.Empty{})
	if err != nil {
		return err
	}

	status, err := statusFromProto(rawStatus)

	if err != nil {
		return err
	}

	s.peers.Store(peerID, &SyncPeer{
		peer:      peerID,
		conn:      conn,
		client:    clt,
		status:    status,
		enqueue:   make(minNumBlockQueue, 0, maxEnqueueSize+1),
		enqueueCh: make(chan struct{}),
	})

	return nil
}

// DeletePeer deletes a peer from syncer
func (s *Syncer) DeletePeer(peerID peer.ID) error {
	p, ok := s.peers.LoadAndDelete(peerID)
	if ok {
		syncPeer, ok := p.(*SyncPeer)
		if !ok {
			return ErrInvalidTypeAssertion
		}

		if err := syncPeer.conn.Close(); err != nil {
			return err
		}

		close(syncPeer.enqueueCh)
	}

	return nil
}

// findCommonAncestor returns the common ancestor header and fork
func (s *Syncer) findCommonAncestor(clt proto.V1Client, status *Status) (*types.Header, *types.Header, error) {
	h := s.blockchain.Header()

	min := uint64(0) // genesis
	max := h.Number

	targetHeight := status.Number

	if heightNumber := targetHeight; max > heightNumber {
		max = heightNumber
	}

	var header *types.Header

	for min <= max {
		m := uint64(math.Floor(float64(min+max) / 2))

		if m == 0 {
			// our common ancestor is the genesis
			genesis, ok := s.blockchain.GetHeaderByNumber(0)
			if !ok {
				return nil, nil, ErrLoadLocalGenesisFailed
			}

			header = genesis

			break
		}

		found, err := getHeader(clt, &m, nil)
		if err != nil {
			return nil, nil, err
		}

		if found == nil {
			// peer does not have the m peer, search in lower bounds
			max = m - 1
		} else {
			expectedHeader, ok := s.blockchain.GetHeaderByNumber(m)
			if !ok {
				return nil, nil, fmt.Errorf("cannot find the header %d in local chain", m)
			}
			if expectedHeader.Hash == found.Hash {
				header = found
				min = m + 1
			} else {
				if m == 0 {
					return nil, nil, ErrMismatchGenesis
				}
				max = m - 1
			}
		}
	}

	if header == nil {
		return nil, nil, ErrCommonAncestorNotFound
	}

	// get the block fork
	forkNum := header.Number + 1
	fork, err := getHeader(clt, &forkNum, nil)

	if err != nil {
		return nil, nil, fmt.Errorf("failed to get fork at num %d", header.Number)
	}

	if fork == nil {
		return nil, nil, ErrForkNotFound
	}

	return header, fork, nil
}

// WatchSyncWithPeer subscribes and adds peer's latest block
func (s *Syncer) WatchSyncWithPeer(
	p *SyncPeer,
	newBlockHandler func(b *types.Block) bool,
	blockTimeout time.Duration,
) {
	// purge from the cache of broadcasted blocks all the ones we have written so far
	header := s.blockchain.Header()
	p.purgeBlocks(header.Hash)

	// listen and enqueue the messages
	for {
		if p.IsClosed() {
			s.logger.Info("Connection to a peer has closed already", "id", p.peer)

			break
		}

		// safe estimate time for fetching new block broadcast
		b, err := p.popBlock(blockTimeout * 3)
		if err != nil {
			s.logSyncPeerPopBlockError(err, p)

			break
		}

		if err := s.blockchain.VerifyFinalizedBlock(b); err != nil {
			s.logger.Error("unable to verify block, %w", err)

			return
		}

		if err := s.blockchain.WriteBlock(b); err != nil {
			s.logger.Error("failed to write block", "err", err)

			break
		}

		shouldExit := newBlockHandler(b)

		s.prunePeerEnqueuedBlocks(b)

		if shouldExit {
			break
		}
	}
}

func (s *Syncer) logSyncPeerPopBlockError(err error, peer *SyncPeer) {
	if errors.Is(err, ErrPopTimeout) {
		msg := "failed to pop block within %ds from peer: id=%s, please check if all the validators are running"
		s.logger.Warn(fmt.Sprintf(msg, int(popTimeout.Seconds()), peer.peer))
	} else {
		s.logger.Info("failed to pop block from peer", "id", peer.peer, "err", err)
	}
}

// BulkSyncWithPeer finds common ancestor with a peer and syncs block until latest block
// Only missing blocks are synced up to the peer's highest block number
func (s *Syncer) BulkSyncWithPeer(p *SyncPeer, newBlockHandler func(block *types.Block)) error {
	// find the common ancestor
	ancestor, fork, err := s.findCommonAncestor(p.client, p.status)
	if err != nil {
		// No need to sync with this peer
		return err
	}

	// find in batches
	s.logger.Debug("fork found", "ancestor", ancestor.Number)

	startBlock := fork

	var (
		lastTarget        uint64
		currentSyncHeight = ancestor.Number + 1
	)

	// Create a blockchain subscription for the sync progression and start tracking
	s.syncProgression.StartProgression(startBlock.Number, s.blockchain.SubscribeEvents())

	// Stop monitoring the sync progression upon exit
	defer s.syncProgression.StopProgression()

	// dynamic modifying syncing size
	blockAmount := int64(maxSkeletonHeadersAmount)

	// sync up to the current known header
	for {
		// Update the target. This entire outer loop
		// is there in order to make sure bulk syncing is entirely done
		// as the peer's status can change over time if block writes have a significant
		// time impact on the node in question
		target := p.status.Number

		s.syncProgression.UpdateHighestProgression(target)

		if target == lastTarget {
			// there are no more changes to pull for now
			break
		}

		for {
			s.logger.Debug(
				"sync up to block",
				"from",
				currentSyncHeight,
				"to",
				target,
			)

			// Create the base request skeleton
			sk := &skeleton{
				amount: blockAmount,
			}

			// Fetch the blocks from the peer
			if err := sk.getBlocksFromPeer(p.client, currentSyncHeight); err != nil {
				if rpcErr, ok := grpcstatus.FromError(err); ok {
					// the data size exceeds grpc server/client message size
					if rpcErr.Code() == grpccodes.ResourceExhausted {
						blockAmount /= 2

						continue
					}
				}

				return fmt.Errorf("unable to fetch blocks from peer, %w", err)
			}

			// increase block amount when succeeded
			blockAmount++
			if blockAmount > maxSkeletonHeadersAmount {
				blockAmount = maxSkeletonHeadersAmount
			}

			// Verify and write the data locally
			for _, block := range sk.blocks {
				if err := s.blockchain.VerifyFinalizedBlock(block); err != nil {
					return fmt.Errorf("unable to verify block, %w", err)
				}

				if err := s.blockchain.WriteBlock(block); err != nil {
					return fmt.Errorf("failed to write block while bulk syncing: %w", err)
				}

				newBlockHandler(block)
				// prune the peers' enqueued block
				s.prunePeerEnqueuedBlocks(block)
				currentSyncHeight++
			}

			if currentSyncHeight >= target {
				// Target has been reached
				break
			}
		}

		lastTarget = target
	}

	return nil
}

func getHeader(clt proto.V1Client, num *uint64, hash *types.Hash) (*types.Header, error) {
	req := &proto.GetHeadersRequest{}
	if num != nil {
		req.Number = int64(*num)
	}

	if hash != nil {
		req.Hash = (*hash).String()
	}

	resp, err := clt.GetHeaders(context.Background(), req)
	if err != nil {
		return nil, err
	}

	if len(resp.Objs) == 0 {
		return nil, nil
	}

	if len(resp.Objs) != 1 {
		return nil, ErrTooManyHeaders
	}

	obj := resp.Objs[0]

	if obj == nil || obj.Spec == nil || len(obj.Spec.Value) == 0 {
		return nil, errNilHeaderResponse
	}

	header := &types.Header{}

	if err := header.UnmarshalRLP(obj.Spec.Value); err != nil {
		return nil, err
	}

	return header, nil
}

func (s *Syncer) prunePeerEnqueuedBlocks(block *types.Block) {
	s.peers.Range(func(key, value interface{}) bool {
		peerID, _ := key.(peer.ID)
		syncPeer, _ := value.(*SyncPeer)

		pruned := syncPeer.purgeBlocks(block.Hash())

		s.logger.Debug(
			"pruned peer enqueued block",
			"num", pruned,
			"id", peerID.String(),
			"reference_block_num", block.Number(),
		)

		return true
	})
}
