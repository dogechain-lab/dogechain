package protocol

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/dogechain-lab/dogechain/blockchain"
	"github.com/dogechain-lab/dogechain/helper/progress"
	"github.com/dogechain-lab/dogechain/network"
	"github.com/dogechain-lab/dogechain/network/event"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/hashicorp/go-hclog"
	"github.com/libp2p/go-libp2p-core/peer"
	"go.uber.org/atomic"
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
	logger hclog.Logger

	blockchain           Blockchain
	blockchainSubscriber blockchain.Subscription

	syncProgression Progression

	peerMap         *PeerMap
	syncPeerService SyncPeerService
	syncPeerClient  SyncPeerClient

	// Channel to notify Sync that a new status arrived
	newStatusCh chan struct{}
	// syncing state
	syncing *atomic.Bool

	// syncing peer id
	syncingPeer *atomic.String

	// stop chan
	stopCh chan struct{}

	// deprecated fields

	// for peer status query
	status     *Status
	statusLock sync.Mutex

	// network server
	server network.Network
	// self node id
	selfID peer.ID
	// broadcasting block flag for backward compatible nodes
	blockBroadcast bool
}

// NewSyncer creates a new Syncer instance
func NewSyncer(
	logger hclog.Logger,
	server network.Network,
	blockchain Blockchain,
	enableBlockBroadcast bool,
) Syncer {
	s := &noForkSyncer{
		logger: logger.Named(_syncerName),

		blockchain:           blockchain,
		blockchainSubscriber: blockchain.SubscribeEvents(),

		syncProgression: progress.NewProgressionWrapper(progress.ChainSyncBulk),
		peerMap:         new(PeerMap),
		syncPeerService: NewSyncPeerService(server, blockchain),
		syncPeerClient:  NewSyncPeerClient(logger, server, blockchain),
		newStatusCh:     make(chan struct{}),
		syncing:         atomic.NewBool(false),
		syncingPeer:     atomic.NewString(""),
		stopCh:          make(chan struct{}),
		server:          server,
		selfID:          server.AddrInfo().ID,
		blockBroadcast:  enableBlockBroadcast,
	}

	// set reference instance
	s.syncPeerService.SetSyncer(s)

	return s
}

func (s *noForkSyncer) isSyncing() bool {
	return s.syncing.Load()
}

func (s *noForkSyncer) setSyncing(syncing bool) (oldStatus bool) {
	return s.syncing.Swap(syncing)
}

// GetSyncProgression returns the latest sync progression, if any
func (s *noForkSyncer) GetSyncProgression() *progress.Progression {
	return s.syncProgression.GetProgression()
}

// runUpdateCurrentStatus taps into the blockchain event steam and updates the Syncer.status field
func (s *noForkSyncer) runUpdateCurrentStatus() {
	// Get the current status of the syncer
	currentHeader := s.blockchain.Header()
	diff, _ := s.blockchain.GetTD(currentHeader.Hash)

	s.status = &Status{
		Hash:       currentHeader.Hash,
		Number:     currentHeader.Number,
		Difficulty: diff,
	}

	// new block event subscription
	sub := s.blockchain.SubscribeEvents()

	updateStatusCh := make(chan *Status, 1)
	defer close(updateStatusCh)

	go func() {
		for {
			select {
			case <-s.stopCh:
				return
			case state, ok := <-updateStatusCh:
				if !ok {
					return
				}

				s.updateStatus(state)
			}
		}
	}()

	// watch the subscription and notify
	for {
		select {
		case <-s.stopCh:
			return
		default:
		}

		if sub.IsClosed() {
			return
		}

		e, ok := <-sub.GetEvent()
		if e == nil || !ok {
			continue
		}

		// we do not want to notify forks
		if e.Type == blockchain.EventFork {
			continue
		}

		// this should not happen
		if len(e.NewChain) == 0 {
			continue
		}

		// skip too many messages
		select {
		case updateStatusCh <- &Status{
			Difficulty: e.Difficulty,
			Hash:       e.NewChain[0].Hash,
			Number:     e.NewChain[0].Number,
		}:
		default:
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
	go s.runUpdateCurrentStatus()

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
	// skip out peers who do not support new version protocol, or IP who could not reach via NAT.
	skipList := new(sync.Map)

	for {
		// Wait for a new event to arrive
		select {
		case <-s.stopCh:
			s.logger.Info("stop syncing")

			return nil
		case _, ok := <-s.newStatusCh:
			// close
			if !ok {
				return nil
			}

			// The channel should not be blocked, otherwise it will hang when an error occurs
			if s.isSyncing() {
				s.logger.Debug("skip new status event due to not done syncing")

				continue
			}
		}

		s.logger.Debug("got new status event")

		if shouldTerminate := s.syncWithSkipList(skipList, callback); shouldTerminate {
			s.logger.Error("terminate syncing")

			break
		}
	}

	return nil
}

func (s *noForkSyncer) syncWithSkipList(
	skipList *sync.Map,
	callback func(*types.Block) bool,
) (shouldTerminate bool) {
	// switch syncing status
	s.setSyncing(true)
	defer s.setSyncing(false)

	s.logger.Debug("start syncing")
	defer s.logger.Debug("done syncing")

	var localLatest uint64

	// fetch local latest block
	if header := s.blockchain.Header(); header != nil {
		localLatest = header.Number
	}

	// pick one best peer
	bestPeer := s.peerMap.BestPeer(skipList)
	if bestPeer == nil {
		s.logger.Info("empty skip list for not getting a best peer")

		if skipList != nil {
			// clear
			skipList.Range(func(key, value interface{}) bool {
				skipList.Delete(key)

				return true
			})
		}

		return
	}

	// if the bestPeer does not have a new block continue
	if bestPeer.Number <= localLatest {
		s.logger.Debug("wait for the best peer catching up the latest block", "bestPeer", bestPeer.ID)

		return
	}

	bestPeerID := bestPeer.ID.String()

	// set up a peer to receive its status updates for progress updates
	s.syncingPeer.Store(bestPeerID)

	// use subscription for updating progression
	s.syncProgression.StartProgression(bestPeerID, localLatest, s.blockchainSubscriber)
	s.syncProgression.UpdateHighestProgression(bestPeer.Number)

	// fetch block from the peer
	result, err := s.bulkSyncWithPeer(bestPeer, callback)
	if err != nil {
		s.logger.Warn("failed to complete bulk sync with peer", "peer ID", bestPeer.ID, "error", err)
	}

	s.logger.Debug("bulk sync with peer done", "peer ID", bestPeer.ID, "result", result)

	// stop progression even it might be not done
	s.syncProgression.StopProgression()
	s.logger.Debug("stop progression")

	// result should never be nil
	for p := range result.SkipList {
		skipList.Store(p, true)
	}

	s.logger.Debug("store result.SkipList")

	return result.ShouldTerminate
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
		from              = s.blockchain.Header().Number + 1
		target            = p.Number
		startBroadcasting bool
	)

	if from > target {
		s.logger.Warn("local header is higher than remote target", "local", from, "remote", target)
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
					s.logger.Debug("peer return recoverable error", "id", p.ID, "err", err)
				default: // other errors are not acceptable
					s.logger.Info("skip peer due to error", "id", p.ID, "err", err)

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

			if newBlockCallback != nil {
				// NOTE: result not use for now, should remove?
				result.ShouldTerminate = newBlockCallback(block)
			}

			result.LastReceivedNumber = block.Number()

			// broadcast latest block to the network
			if s.blockBroadcast && blockNearEnough(block.Number(), target) {
				startBroadcasting = true // upgrade broadcasting flag
			}

			// After switching to broadcast, we don't close it until it catches up or returns an error
			if startBroadcasting {
				s.logger.Info("broadcast block and status", "height", result.LastReceivedNumber)
				s.syncPeerClient.Broadcast(block)
			}
		}

		// update range
		from = result.LastReceivedNumber + 1

		// Update the target. This entire outer loop is there in order to make sure
		// bulk syncing is entirely done as the peer's status can change over time
		// if block writes have a significant time impact on the node in question
		progression := s.syncProgression.GetProgression()
		if progression != nil {
			if highestBlock := progression.GetHighestBlock(); highestBlock > target {
				target = highestBlock
				s.logger.Debug("update syncing target", "target", target)
			}
		}

		if from > target {
			s.logger.Info("sync target reached", "target", target)

			break
		}
	}

	return result, nil
}

func blockNearEnough(a, b uint64) bool {
	const nearBlockHeight = 1

	switch a >= b {
	case true:
		return a-b <= nearBlockHeight
	default:
		return b-a <= nearBlockHeight
	}
}

// initializePeerMap fetches peer statuses and initializes map
func (s *noForkSyncer) initializePeerMap() {
	peerStatuses := s.syncPeerClient.GetConnectedPeerStatuses()
	s.peerMap.Put(peerStatuses...)
}

// startPeerStatusUpdateProcess subscribes peer status change event and updates peer map
func (s *noForkSyncer) startPeerStatusUpdateProcess() {
	for peerStatus := range s.syncPeerClient.GetPeerStatusUpdateCh() {
		s.logger.Debug("peer status updated", "id", peerStatus.ID, "number", peerStatus.Number)
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
	s.logger.Info("peer connected", "id", peerID)

	status, err := s.syncPeerClient.GetPeerStatus(peerID)
	if err != nil {
		s.logger.Warn("failed to get peer status, skip", "id", peerID, "err", err)

		status = &NoForkPeer{
			ID: peerID,
		}
	}

	// update its status
	s.putToPeerMap(status)
}

// putToPeerMap puts given status to peer map
func (s *noForkSyncer) putToPeerMap(status *NoForkPeer) {
	if status == nil {
		// it should not be
		return
	}

	if !s.peerMap.Exists(status.ID) {
		s.logger.Info("new connected peer", "id", status.ID, "number", status.Number)
	}

	if status.ID == s.selfID {
		// skip self
		return
	}

	// copy syncing peer id
	syncingPeer := s.syncingPeer.Load()

	s.logger.Debug("syncingPeer", "id", syncingPeer, "status.ID", status.ID.String())

	// update progression if needed
	if status.ID.String() == syncingPeer && status.Number > 0 {
		s.logger.Debug("connected peer update status", "id", status.ID, "number", status.Number)
		s.syncProgression.UpdateHighestProgression(status.Number)
	}

	s.peerMap.Put(status)
	s.notifyNewStatusEvent()
}

// removeFromPeerMap removes the peer from peer map
func (s *noForkSyncer) removeFromPeerMap(peerID peer.ID) {
	s.logger.Info("remove from peer map", "id", peerID)

	s.peerMap.Remove(peerID)
}

// notifyNewStatusEvent emits signal to newStatusCh
func (s *noForkSyncer) notifyNewStatusEvent() {
	select {
	case s.newStatusCh <- struct{}{}:
	default:
	}
}
