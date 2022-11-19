package ibft

import (
	"container/heap"
	"sync"

	"github.com/dogechain-lab/dogechain/consensus/ibft/currentstate"
	"github.com/dogechain-lab/dogechain/consensus/ibft/proto"
)

// msgQueue defines the structure that holds message queues for different IBFT states
type msgQueue struct {
	// Heap implementation for the round change message queue
	roundChangeStateQueue msgQueueImpl

	// Heap implementation for the accept state message queue
	acceptStateQueue msgQueueImpl

	// Heap implementation for the validate state message queue
	validateStateQueue msgQueueImpl

	queueLock sync.Mutex
}

// pushMessage adds a new message to a message queue
func (m *msgQueue) pushMessage(task *msgTask) {
	m.queueLock.Lock()

	queue := m.getQueue(msgToState(task.msg))
	heap.Push(queue, task)

	m.queueLock.Unlock()
}

// readMessage reads the message from a message queue, based on the current state and view
func (m *msgQueue) readMessage(state currentstate.IbftState, current *proto.View) *msgTask {
	m.queueLock.Lock()
	defer m.queueLock.Unlock()

	queue := m.getQueue(state)

	for {
		if queue.Len() == 0 {
			return nil
		}

		msg := queue.head()

		// check if the message is from the future
		if state == currentstate.RoundChangeState {
			// if we are in RoundChangeState we only care about sequence
			// since we are interested in knowing all the possible rounds
			if msg.view.Sequence > current.Sequence {
				// future message
				return nil
			}
		} else {
			// otherwise, we compare both sequence and round
			if cmpView(msg.view, current) > 0 {
				// future message
				return nil
			}
		}

		// at this point, 'msg' is good or old, in either case
		// we have to remove it from the queue
		heap.Pop(queue)

		if cmpView(msg.view, current) < 0 {
			// old value, try again
			continue
		}

		// good value, return it
		return msg
	}
}

// getQueue checks the passed in state, and returns the corresponding message queue
func (m *msgQueue) getQueue(state currentstate.IbftState) *msgQueueImpl {
	if state == currentstate.RoundChangeState {
		// round change
		return &m.roundChangeStateQueue
	} else if state == currentstate.AcceptState {
		// preprepare
		return &m.acceptStateQueue
	} else {
		// prepare, commit and post commit
		return &m.validateStateQueue
	}
}

// newMsgQueue creates a new message queue structure
func newMsgQueue() *msgQueue {
	return &msgQueue{
		roundChangeStateQueue: msgQueueImpl{},
		acceptStateQueue:      msgQueueImpl{},
		validateStateQueue:    msgQueueImpl{},
	}
}

// protoTypeToMsg converts the proto message request type to a MsgType object
func protoTypeToMsg(msgType proto.MessageReq_Type) MsgType {
	switch msgType {
	case proto.MessageReq_Preprepare:
		return msgPreprepare
	case proto.MessageReq_Prepare:
		return msgPrepare
	case proto.MessageReq_Commit:
		return msgCommit
	case proto.MessageReq_PostCommit:
		return msgPostCommit
	default:
		return msgRoundChange
	}
}

// msgToState converts the message type to an IbftState
func msgToState(msg MsgType) currentstate.IbftState {
	switch msg {
	case msgRoundChange:
		// round change
		return currentstate.RoundChangeState
	case msgPreprepare:
		// preprepare
		return currentstate.AcceptState
	case msgPrepare, msgCommit, msgPostCommit:
		// prepare, commit and post commit
		return currentstate.ValidateState
	}

	panic("BUG: not expected")
}

type MsgType uint64

// Define message types
const (
	// priority order for the messages
	msgRoundChange MsgType = 0
	msgPreprepare  MsgType = 1
	msgCommit      MsgType = 2
	msgPrepare     MsgType = 3
	msgPostCommit  MsgType = 4
)

// String returns the string representation of the message type
func (m MsgType) String() string {
	switch m {
	case msgRoundChange:
		return "RoundChange"
	case msgPrepare:
		return "Prepare"
	case msgPreprepare:
		return "Preprepare"
	case msgCommit:
		return "Commit"
	case msgPostCommit:
		return "PostCommit"
	default:
		panic("BUG")
	}
}

type msgTask struct {
	// priority
	view *proto.View
	msg  MsgType

	obj *proto.MessageReq
}

type msgQueueImpl []*msgTask

// head returns the head of the queue
func (m msgQueueImpl) head() *msgTask {
	return m[0]
}

// Len returns the length of the queue
func (m msgQueueImpl) Len() int {
	return len(m)
}

// Less compares the priorities of two items at the passed in indexes (A < B)
func (m msgQueueImpl) Less(i, j int) bool {
	ti, tj := m[i], m[j]
	// sort by sequence
	if ti.view.Sequence != tj.view.Sequence {
		return ti.view.Sequence < tj.view.Sequence
	}
	// sort by round
	if ti.view.Round != tj.view.Round {
		return ti.view.Round < tj.view.Round
	}
	// sort by message
	return ti.msg < tj.msg
}

// Swap swaps the places of the items at the passed-in indexes
func (m msgQueueImpl) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

// Push adds a new item to the queue
func (m *msgQueueImpl) Push(x interface{}) {
	task, ok := x.(*msgTask)
	if !ok {
		return
	}

	*m = append(*m, task)
}

// Pop removes an item from the queue
func (m *msgQueueImpl) Pop() interface{} {
	old := *m
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*m = old[0 : n-1]

	return item
}

// cmpView compares two proto views.
//
// If v.Sequence == y.Sequence && v.Round == y.Round => 0
//
// If v.Sequence < y.Sequence => -1 ELSE => 1
//
// If v.Round < y.Round => -1 ELSE 1
func cmpView(v, y *proto.View) int {
	if v.Sequence != y.Sequence {
		if v.Sequence < y.Sequence {
			return -1
		} else {
			return 1
		}
	}

	if v.Round != y.Round {
		if v.Round < y.Round {
			return -1
		} else {
			return 1
		}
	}

	return 0
}
