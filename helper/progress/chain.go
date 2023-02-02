package progress

import (
	"sync"

	"github.com/dogechain-lab/dogechain/blockchain"
	"go.uber.org/atomic"
)

type ChainSyncType string

const (
	ChainSyncRestore ChainSyncType = "restore"
	ChainSyncBulk    ChainSyncType = "bulk-sync"
)

// Progression defines the status of the sync
// progression of the node
type Progression struct {
	// SyncType is indicating the sync method
	SyncType ChainSyncType

	// SyncingPeer is current syncing peer id
	SyncingPeer string

	// StartingBlock is the initial block that the node is starting
	// the sync from. It is reset after every sync batch
	StartingBlock uint64

	// CurrentBlock is the last written block from the sync batch
	CurrentBlock uint64

	// HighestBlock is the target block in the sync batch
	HighestBlock *atomic.Uint64

	// stopCh is the channel for receiving stop signals
	// in progression tracking
	stopCh chan struct{}

	// stop flag
	stopped *atomic.Bool
}

type ProgressionWrapper struct {
	// progression is a reference to the ongoing batch sync.
	// Nil if no batch sync is currently in progress
	progression *Progression

	// sync lock
	lock sync.RWMutex

	syncType ChainSyncType
}

func NewProgressionWrapper(syncType ChainSyncType) *ProgressionWrapper {
	return &ProgressionWrapper{
		progression: nil,
		syncType:    syncType,
	}
}

// startProgression initializes the progression tracking
func (pw *ProgressionWrapper) StartProgression(
	syncingPeer string,
	startingBlock uint64,
	subscription blockchain.Subscription,
) {
	pw.lock.Lock()
	defer pw.lock.Unlock()

	// clear previous progression
	pw.clearProgression()

	// set current block
	var current uint64

	if startingBlock > 0 {
		current = startingBlock - 1
	}

	pw.progression = &Progression{
		SyncType:      pw.syncType,
		SyncingPeer:   syncingPeer,
		StartingBlock: startingBlock,
		CurrentBlock:  current,
		HighestBlock:  atomic.NewUint64(0),
		stopCh:        make(chan struct{}),
		stopped:       atomic.NewBool(false),
	}

	go pw.progression.runUpdateLoop(subscription)
}

// runUpdateLoop starts the blockchain event monitoring loop and
// updates the currently written block in the batch sync
func (p *Progression) runUpdateLoop(subscription blockchain.Subscription) {
	for {
		select {
		case <-p.stopCh:
			return
		default:
			if subscription.IsClosed() {
				continue
			}

			event, ok := <-subscription.GetEvent()
			if event == nil || p.stopped.Load() || !ok {
				continue
			}

			if event.Type == blockchain.EventFork {
				continue
			}

			if len(event.NewChain) == 0 {
				continue
			}

			lastBlock := event.NewChain[len(event.NewChain)-1]
			p.HighestBlock.Store(lastBlock.Number)
		}
	}
}

func (p *Progression) GetHighestBlock() uint64 {
	return p.HighestBlock.Load()
}

// clearProgression clear progression object (non-thread safe)
func (pw *ProgressionWrapper) clearProgression() {
	if pw.progression != nil && pw.progression.stopped.CAS(false, true) {
		close(pw.progression.stopCh)
		pw.progression = nil
	}
}

// StopProgression stops the progression tracking
func (pw *ProgressionWrapper) StopProgression() {
	pw.lock.Lock()
	defer pw.lock.Unlock()

	pw.clearProgression()
}

// UpdateCurrentProgression sets the currently written block in the bulk sync
func (pw *ProgressionWrapper) UpdateCurrentProgression(currentBlock uint64) {
	pw.lock.Lock()
	defer pw.lock.Unlock()

	if pw.progression == nil {
		return
	}

	pw.progression.CurrentBlock = currentBlock
}

// UpdateHighestProgression sets the highest-known target block in the bulk sync
func (pw *ProgressionWrapper) UpdateHighestProgression(highestBlock uint64) {
	pw.lock.Lock()
	defer pw.lock.Unlock()

	if pw.progression == nil {
		return
	}

	pw.progression.HighestBlock.Store(highestBlock)
}

// GetProgression returns the latest sync progression
func (pw *ProgressionWrapper) GetProgression() *Progression {
	pw.lock.RLock()
	defer pw.lock.RUnlock()

	return pw.progression
}
