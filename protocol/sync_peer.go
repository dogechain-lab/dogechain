package protocol

import (
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/dogechain-lab/dogechain/protocol/proto"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/libp2p/go-libp2p-core/peer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// Status defines the up to date information regarding the peer
type Status struct {
	Difficulty *big.Int   // Current difficulty
	Hash       types.Hash // Latest block hash
	Number     uint64     // Latest block number
}

// Copy creates a copy of the status
func (s *Status) Copy() *Status {
	ss := new(Status)
	ss.Hash = s.Hash
	ss.Number = s.Number
	ss.Difficulty = new(big.Int).Set(s.Difficulty)

	return ss
}

// toProto converts a Status object to a proto.V1Status
func (s *Status) toProto() *proto.V1Status {
	return &proto.V1Status{
		Number:     s.Number,
		Hash:       s.Hash.String(),
		Difficulty: s.Difficulty.String(),
	}
}

// statusFromProto extracts a Status object from a passed in proto.V1Status
func statusFromProto(p *proto.V1Status) (*Status, error) {
	s := &Status{
		Hash:   types.StringToHash(p.Hash),
		Number: p.Number,
	}

	diff, ok := new(big.Int).SetString(p.Difficulty, 10)
	if !ok {
		return nil, ErrDecodeDifficulty
	}

	s.Difficulty = diff

	return s, nil
}

// SyncPeer is a representation of the peer the node is syncing with
type SyncPeer struct {
	peer   peer.ID
	conn   *grpc.ClientConn
	client proto.V1Client

	// Peer status might not be the latest block due to its asynchronous broadcast
	// mechanism. The goroutine would makes the sequence unpredictable.
	// So do not rely on its status for step by step watching syncing, especially
	// in a bad network status.
	// We would rather evolve the syncing protocol instead of patching too much for
	// v1 protocol.
	status     *Status
	statusLock sync.RWMutex

	enqueueLock sync.Mutex
	enqueue     minNumBlockQueue
	enqueueCh   chan struct{}
}

// Number returns the latest peer block height
func (s *SyncPeer) Number() uint64 {
	s.statusLock.RLock()
	defer s.statusLock.RUnlock()

	return s.status.Number
}

// IsClosed returns whether peer's connectivity has been closed
func (s *SyncPeer) IsClosed() bool {
	return s.conn.GetState() == connectivity.Shutdown
}

// purgeBlocks purges the cache of broadcasted blocks the node has written so far
// from the SyncPeer
func (s *SyncPeer) purgeBlocks(lastSeen types.Hash) uint64 {
	s.enqueueLock.Lock()
	defer s.enqueueLock.Unlock()

	index := -1

	for i, b := range s.enqueue {
		if b.Hash() == lastSeen {
			index = i

			break
		}
	}

	if index == -1 {
		// no blocks enqueued
		return 0
	}

	s.enqueue = s.enqueue[index+1:]

	return uint64(index + 1)
}

// popBlock pops a block from the block queue [BLOCKING]
func (s *SyncPeer) popBlock(timeout time.Duration) (b *types.Block, err error) {
	timeoutCh := time.NewTimer(timeout)

	for {
		if !s.IsClosed() {
			s.enqueueLock.Lock()
			if len(s.enqueue) != 0 {
				b, s.enqueue = s.enqueue[0], s.enqueue[1:]
				s.enqueueLock.Unlock()

				return
			}

			s.enqueueLock.Unlock()
			select {
			case <-s.enqueueCh:
			case <-timeoutCh.C:
				return nil, ErrPopTimeout
			}
		} else {
			return nil, ErrConnectionClosed
		}
	}
}

// appendBlock adds a new block to the block queue
func (s *SyncPeer) appendBlock(b *types.Block) {
	s.enqueueLock.Lock()
	defer s.enqueueLock.Unlock()

	// append block
	s.enqueue = append(s.enqueue, b)
	// sort blocks
	sort.Stable(&s.enqueue)

	if s.enqueue.Len() > maxEnqueueSize {
		// pop elements to meet capacity
		s.enqueue = s.enqueue[s.enqueue.Len()-maxEnqueueSize:]
	}

	select {
	case s.enqueueCh <- struct{}{}:
	default:
	}
}

func (s *SyncPeer) updateStatus(status *Status) {
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

	s.status = status
}
