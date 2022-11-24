package protocol

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/dogechain-lab/dogechain/blockchain"
	cmap "github.com/dogechain-lab/dogechain/helper/concurrentmap"
	"github.com/dogechain-lab/dogechain/helper/progress"
	"github.com/dogechain-lab/dogechain/network"
	"github.com/dogechain-lab/dogechain/network/event"
	"github.com/dogechain-lab/dogechain/protocol/proto"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/hashicorp/go-hclog"
	"github.com/libp2p/go-libp2p-core/peer"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	anypb "google.golang.org/protobuf/types/known/anypb"
)

const (
	_syncerName = "syncer"
	// version not change for backward compatibility
	_syncerV1 = "/syncer/0.1"
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

	errTimeout = errors.New("timeout awaiting block from peer")
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

// noForkSyncer is an implementation for Syncer Protocol
//
// NOTE: Do not use this syncer for the consensus that may cause fork.
// This syncer doesn't assume forks
type noForkSyncer struct {
	logger          hclog.Logger
	blockchain      Blockchain
	syncProgression Progression

	peerMap         *PeerMap
	syncPeerService SyncPeerService
	syncPeerClient  SyncPeerClient

	blockTimeout time.Duration

	// Channel to notify Sync that a new status arrived
	newStatusCh chan struct{}

	// stop chan
	stopCh chan struct{}

	// deprecated fields

	// Maps peer.ID -> SyncPeer
	peers      cmap.ConcurrentMap
	status     *Status
	statusLock sync.Mutex

	server *network.Server
}

// NewSyncer creates a new Syncer instance
func NewSyncer(
	logger hclog.Logger,
	server *network.Server,
	blockchain Blockchain,
	blockTimeout time.Duration,
) Syncer {
	s := &noForkSyncer{
		logger:          logger.Named(_syncerName),
		blockchain:      blockchain,
		syncProgression: progress.NewProgressionWrapper(progress.ChainSyncBulk),
		peers:           cmap.NewConcurrentMap(),
		peerMap:         new(PeerMap),
		syncPeerService: NewSyncPeerService(server, blockchain),
		syncPeerClient:  NewSyncPeerClient(logger, server, blockchain),
		blockTimeout:    blockTimeout,
		newStatusCh:     make(chan struct{}),
		stopCh:          make(chan struct{}),
		server:          server,
	}

	// set reference instance
	s.syncPeerService.SetSyncer(s)

	return s
}

// GetSyncProgression returns the latest sync progression, if any
func (s *noForkSyncer) GetSyncProgression() *progress.Progression {
	return s.syncProgression.GetProgression()
}

// updateCurrentStatus taps into the blockchain event steam and updates the Syncer.status field
func (s *noForkSyncer) updateCurrentStatus() {
	// Get the current status of the syncer
	currentHeader := s.blockchain.Header()
	diff, _ := s.blockchain.GetTD(currentHeader.Hash)

	s.status = &Status{
		Hash:       currentHeader.Hash,
		Number:     currentHeader.Number,
		Difficulty: diff,
	}

	sub := s.blockchain.SubscribeEvents()
	defer sub.Close()

	// watch the subscription and notify
	for {
		select {
		case evnt := <-sub.GetEventCh():
			// we do not want to notify forks
			if evnt.Type == blockchain.EventFork {
				continue
			}

			// this should not happen
			if len(evnt.NewChain) == 0 {
				continue
			}

			s.updateStatus(&Status{
				Difficulty: evnt.Difficulty,
				Hash:       evnt.NewChain[0].Hash,
				Number:     evnt.NewChain[0].Number,
			})
		case <-s.stopCh:
			return
		}
	}
}

func (s *noForkSyncer) updateStatus(status *Status) {
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

// enqueueBlock adds the specific block to the peerID queue
func (s *noForkSyncer) enqueueBlock(peerID peer.ID, b *types.Block) {
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

func (s *noForkSyncer) updatePeerStatus(peerID peer.ID, status *Status) {
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
func (s *noForkSyncer) Broadcast(b *types.Block) {
	sendNotify := func(peerID, peer interface{}, req *proto.NotifyReq) {
		startTime := time.Now()

		syncPeer, ok := peer.(*SyncPeer)
		if !ok {
			return
		}

		if _, err := syncPeer.client.Notify(context.Background(), req); err != nil {
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
func (s *noForkSyncer) Start() error {
	if err := s.syncPeerClient.Start(); err != nil {
		return err
	}

	s.syncPeerService.Start()

	// init peer list
	s.initializePeerMap()

	// process
	go s.startPeerStatusUpdateProcess()
	go s.startPeerConnectionEventProcess()

	// Run the blockchain event listener loop
	// deprecated, only for backward compatibility
	go s.updateCurrentStatus()

	return nil
}

func (s *noForkSyncer) Close() error {
	close(s.stopCh)

	if err := s.syncPeerService.Close(); err != nil {
		return err
	}

	return nil
}

// HasSyncPeer returns whether syncer has the peer to syncs blocks
// return false if syncer has no peer whose latest block height doesn't exceed local height
func (s *noForkSyncer) HasSyncPeer() bool {
	bestPeer := s.peerMap.BestPeer(nil)
	header := s.blockchain.Header()

	return bestPeer != nil && bestPeer.Number > header.Number
}

// Sync syncs block with the best peer until callback returns true
func (s *noForkSyncer) Sync(callback func(*types.Block) bool) error {
	localLatest := s.blockchain.Header().Number
	// skip out peers who do not support new version protocol, or IP who could not reach via NAT.
	skipList := make(map[peer.ID]bool)

	for {
		// Wait for a new event to arrive
		select {
		case <-s.stopCh:
			s.logger.Info("stop syncing")

			return nil
		case <-s.newStatusCh:
		}

		// fetch local latest block
		if header := s.blockchain.Header(); header != nil {
			localLatest = header.Number
		}

		// pick one best peer
		bestPeer := s.peerMap.BestPeer(skipList)
		if bestPeer == nil {
			s.logger.Info("empty skip list for not getting a best peer")

			skipList = make(map[peer.ID]bool)

			continue
		}

		// if the bestPeer does not have a new block continue
		if bestPeer.Number <= localLatest {
			s.logger.Info("wait for the best peer catching up the latest block", "bestPeer", bestPeer.ID)

			continue
		}

		// fetch block from the peer
		lastNumber, shouldTerminate, err := s.bulkSyncWithPeer(bestPeer.ID, callback)
		if err != nil {
			s.logger.Warn("failed to complete bulk sync with peer, try to next one", "peer ID", "error", bestPeer.ID, err)
		}

		if lastNumber < bestPeer.Number {
			skipList[bestPeer.ID] = true

			// continue to next peer
			continue
		}

		if shouldTerminate {
			break
		}
	}

	return nil
}

// bulkSyncWithPeer syncs block with a given peer
func (s *noForkSyncer) bulkSyncWithPeer(
	peerID peer.ID,
	newBlockCallback func(*types.Block) bool,
) (uint64, bool, error) {
	localLatest := s.blockchain.Header().Number
	shouldTerminate := false

	blockCh, err := s.syncPeerClient.GetBlocks(peerID, localLatest+1, s.blockTimeout)
	if err != nil {
		return 0, false, err
	}

	defer func() {
		if err := s.syncPeerClient.CloseStream(peerID); err != nil {
			s.logger.Error("Failed to close stream: ", err)
		}
	}()

	var (
		lastReceivedNumber uint64
		timer              = time.NewTimer(s.blockTimeout)
	)

	defer timer.Stop()

	for {
		select {
		case block, ok := <-blockCh:
			if !ok {
				return lastReceivedNumber, shouldTerminate, nil
			}

			// safe check
			if block.Number() == 0 {
				continue
			}

			if err := s.blockchain.VerifyFinalizedBlock(block); err != nil {
				return lastReceivedNumber, false, fmt.Errorf("unable to verify block, %w", err)
			}

			if err := s.blockchain.WriteBlock(block); err != nil {
				return lastReceivedNumber, false, fmt.Errorf("failed to write block while bulk syncing: %w", err)
			}

			shouldTerminate = newBlockCallback(block)

			lastReceivedNumber = block.Number()
		case <-timer.C:
			return lastReceivedNumber, shouldTerminate, errTimeout
		}
	}
}

// BestPeer returns the best peer by difficulty (if any)
func (s *noForkSyncer) BestPeer() *SyncPeer {
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
		// compare block height
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

// WatchSyncWithPeer subscribes and adds peer's latest block
func (s *noForkSyncer) WatchSyncWithPeer(
	p *SyncPeer,
	newBlockHandler func(b *types.Block) bool,
	blockTimeout time.Duration,
) {
	// purge from the cache of broadcasted blocks all the ones we have written so far
	header := s.blockchain.Header()
	p.purgeBlocks(header.Hash)

	// localLatest := header.Number
	// shouldTerminate := false

	// listen and enqueue the messages
	for {
		if p.IsClosed() {
			s.logger.Info("Connection to a peer has closed already", "id", p.ID())

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

func (s *noForkSyncer) logSyncPeerPopBlockError(err error, peer *SyncPeer) {
	if errors.Is(err, ErrPopTimeout) {
		msg := "failed to pop block within %ds from peer: id=%s, please check if all the validators are running"
		s.logger.Warn(fmt.Sprintf(msg, int(popTimeout.Seconds()), peer.ID()))
	} else {
		s.logger.Info("failed to pop block from peer", "id", peer.ID(), "err", err)
	}
}

// BulkSyncWithPeer syncs block with a given peer
//
// No need to find common ancestor, download blocks and execute it concurrently,
// drop peer connection when execute failed.
// Only missing blocks are synced up to the peer's highest block number
func (s *noForkSyncer) BulkSyncWithPeer(p *SyncPeer, newBlockHandler func(block *types.Block)) error {
	logger := s.logger.Named("bulkSync")

	localLatest := s.blockchain.Header().Number

	var (
		lastTarget        uint64
		currentSyncHeight = localLatest + 1
	)

	// Create a blockchain subscription for the sync progression and start tracking
	s.syncProgression.StartProgression(localLatest, s.blockchain.SubscribeEvents())

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
			logger.Info("catch up, no need to bulk sync now")

			break
		}

		if !p.IsForwardable() {
			logger.Info("peer is not forwardable", "peer", p.ID())

			break
		}

		for {
			logger.Info(
				"sync up to block",
				"peer", p.ID(),
				"from", currentSyncHeight,
				"to", target,
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
					s.server.DisconnectFromPeer(p.ID(), "Different network due to hard fork")

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

func (s *noForkSyncer) prunePeerEnqueuedBlocks(block *types.Block) {
	s.peers.Range(func(key, value interface{}) bool {
		peerID, ok := key.(peer.ID)
		if !ok {
			return true
		}

		syncPeer, ok := value.(*SyncPeer)
		if !ok {
			return true
		}

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

// initializePeerMap fetches peer statuses and initializes map
func (s *noForkSyncer) initializePeerMap() {
	peerStatuses := s.syncPeerClient.GetConnectedPeerStatuses()
	s.peerMap.Put(peerStatuses...)
}

// startPeerStatusUpdateProcess subscribes peer status change event and updates peer map
func (s *noForkSyncer) startPeerStatusUpdateProcess() {
	for peerStatus := range s.syncPeerClient.GetPeerStatusUpdateCh() {
		s.putToPeerMap(peerStatus)
	}
}

// startPeerConnectionEventProcess processes peer connection change events
func (s *noForkSyncer) startPeerConnectionEventProcess() {
	for e := range s.syncPeerClient.GetPeerConnectionUpdateEventCh() {
		peerID := e.PeerID

		switch e.Type {
		case event.PeerConnected:
			go s.initNewPeerStatus(peerID)
		case event.PeerDisconnected:
			s.removeFromPeerMap(peerID)
		}
	}
}

// initNewPeerStatus fetches status of the peer and put to peer map
func (s *noForkSyncer) initNewPeerStatus(peerID peer.ID) {
	status, err := s.syncPeerClient.GetPeerStatus(peerID)
	if err != nil {
		s.logger.Warn("failed to get peer status, skip", "id", peerID, "err", err)

		return
	}

	s.putToPeerMap(status)
}

// putToPeerMap puts given status to peer map
func (s *noForkSyncer) putToPeerMap(status *NoForkPeer) {
	s.peerMap.Put(status)
	s.notifyNewStatusEvent()
}

// removeFromPeerMap removes the peer from peer map
func (s *noForkSyncer) removeFromPeerMap(peerID peer.ID) {
	s.peerMap.Remove(peerID)
}

// notifyNewStatusEvent emits signal to newStatusCh
func (s *noForkSyncer) notifyNewStatusEvent() {
	select {
	case s.newStatusCh <- struct{}{}:
	default:
	}
}
