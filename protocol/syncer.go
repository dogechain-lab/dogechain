package protocol

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/dogechain-lab/dogechain/blockchain"
	"github.com/dogechain-lab/dogechain/helper/progress"
	"github.com/dogechain-lab/dogechain/network"
	"github.com/dogechain-lab/dogechain/network/event"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/hashicorp/go-hclog"
	"github.com/libp2p/go-libp2p-core/peer"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

const (
	_syncerName = "syncer"
	// version not change for backward compatibility
	_syncerV1 = "/syncer/0.1"

	WriteBlockSource = "syncer"

	// One step query blocks.
	// Median rlp block size is around 20 - 50 KB, then 2 - 4 MB is suitable for one query.
	_blockSyncStep = 100
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

	// for peer status query
	status     *Status
	statusLock sync.Mutex
	// network server
	server *network.Server
	// broadcasting block flag for backward compatible nodes
	blockBroadcast bool
}

// NewSyncer creates a new Syncer instance
func NewSyncer(
	logger hclog.Logger,
	server *network.Server,
	blockchain Blockchain,
	blockTimeout time.Duration,
	enableBlockBroadcast bool,
) Syncer {
	s := &noForkSyncer{
		logger:          logger.Named(_syncerName),
		blockchain:      blockchain,
		syncProgression: progress.NewProgressionWrapper(progress.ChainSyncBulk),
		peerMap:         new(PeerMap),
		syncPeerService: NewSyncPeerService(server, blockchain),
		syncPeerClient:  NewSyncPeerClient(logger, server, blockchain),
		blockTimeout:    blockTimeout,
		newStatusCh:     make(chan struct{}),
		stopCh:          make(chan struct{}),
		server:          server,
		blockBroadcast:  enableBlockBroadcast,
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
			s.logger.Debug("wait for the best peer catching up the latest block", "bestPeer", bestPeer.ID)

			continue
		}

		// use subscription for updating progression
		s.syncProgression.StartProgression(localLatest, s.blockchain.SubscribeEvents())
		s.syncProgression.UpdateHighestProgression(bestPeer.Number)

		// fetch block from the peer
		result, err := s.bulkSyncWithPeer(bestPeer, callback)
		if err != nil {
			s.logger.Warn("failed to complete bulk sync with peer, try to next one", "peer ID", "error", bestPeer.ID, err)
		}

		// stop progression even it might be not done
		s.syncProgression.StopProgression()

		// result should never be nil
		for p := range result.SkipList {
			skipList[p] = true
		}

		if result.ShouldTerminate {
			break
		}

		if err == nil && s.blockBroadcast {
			b, ok := s.blockchain.GetBlockByNumber(result.LastReceivedNumber, true)
			if ok {
				s.logger.Info("broadcast block and status", "height", result.LastReceivedNumber)
				s.syncPeerClient.Broadcast(b)
			}
		}
	}

	return nil
}

type bulkSyncResult struct {
	SkipList           map[peer.ID]bool
	LastReceivedNumber uint64
	ShouldTerminate    bool
}

// bulkSyncWithPeer syncs block with a given peer
func (s *noForkSyncer) bulkSyncWithPeer(
	p *NoForkPeer,
	newBlockCallback func(*types.Block) bool,
) (*bulkSyncResult, error) {
	var (
		result = &bulkSyncResult{
			SkipList:           make(map[peer.ID]bool),
			LastReceivedNumber: 0,
			ShouldTerminate:    false,
		}
		from   = s.blockchain.Header().Number + 1
		target = p.Number
	)

	if from > target {
		// it should not be
		return result, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// sync up to the current known header
	for {
		// set to
		to := from + _blockSyncStep - 1
		if to > target {
			// adjust to
			to = target
		}

		s.logger.Info("sync up to block", "peer", p.ID, "from", from, "to", to)

		blocks, err := s.syncPeerClient.GetBlocks(ctx, p.ID, from, to)
		if err != nil {
			if rpcErr, ok := grpcstatus.FromError(err); ok {
				switch rpcErr.Code() {
				case grpccodes.OK, grpccodes.Canceled, grpccodes.DataLoss:
				default: // other errors are not acceptable
					result.SkipList[p.ID] = true
				}
			}

			return result, err
		}

		if len(blocks) > 0 {
			s.logger.Info(
				"get all blocks",
				"peer", p.ID,
				"from", blocks[0].Number(),
				"to", blocks[len(blocks)-1].Number())
		}

		// write block
		for _, block := range blocks {
			if err := s.blockchain.VerifyFinalizedBlock(block); err != nil {
				// not the same network
				result.SkipList[p.ID] = true

				return result, fmt.Errorf("unable to verify block, %w", err)
			}

			if err := s.blockchain.WriteBlock(block, WriteBlockSource); err != nil {
				return result, fmt.Errorf("failed to write block while bulk syncing: %w", err)
			}

			// NOTE: not use for now, should remove?
			result.ShouldTerminate = newBlockCallback(block)
			result.LastReceivedNumber = block.Number()
		}

		// update range
		from = result.LastReceivedNumber + 1

		// Update the target. This entire outer loop is there in order to make sure
		// bulk syncing is entirely done as the peer's status can change over time
		// if block writes have a significant time impact on the node in question
		progression := s.syncProgression.GetProgression()
		if progression != nil && progression.HighestBlock > target {
			target = progression.HighestBlock
			s.logger.Debug("update syncing target", "target", target)
		}

		if from > target {
			s.logger.Debug("sync target reached", "target", target)

			break
		}
	}

	return result, nil
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
	// update progression if OK
	if p := s.syncProgression; p != nil && status != nil {
		p.UpdateHighestProgression(status.Number)
	}

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
	default: // OK to skip event when chan block
	}
}
