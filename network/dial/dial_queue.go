package dial

import (
	"container/heap"
	"sync"

	"github.com/dogechain-lab/dogechain/network/common"
	"go.uber.org/atomic"

	"github.com/libp2p/go-libp2p/core/peer"
)

// DialQueue is a queue that holds dials tasks for potential peers, implemented as a min-heap
type DialQueue struct {
	sync.Mutex

	heap  dialQueueImpl
	tasks map[peer.ID]*DialTask

	updateCh chan struct{}

	closed  *atomic.Bool
	closeCh chan struct{}
}

// NewDialQueue creates a new DialQueue instance
func NewDialQueue() *DialQueue {
	return &DialQueue{
		heap:     dialQueueImpl{},
		tasks:    map[peer.ID]*DialTask{},
		updateCh: make(chan struct{}),
		closed:   atomic.NewBool(false),
		closeCh:  make(chan struct{}),
	}
}

// Close closes the running DialQueue
func (d *DialQueue) Close() {
	d.closed.CAS(false, true)

	// return PopTask call
	select {
	case d.closeCh <- struct{}{}:
	default:
	}
}

// PopTask is a loop that handles update and close events [BLOCKING]
func (d *DialQueue) PopTask() *DialTask {
	for {
		if d.closed.Load() {
			return nil
		}

		task := d.popTaskImpl() // Blocking pop
		if task != nil {
			return task
		}

		// if task is nil, wait next update or close event
		select {
		case <-d.updateCh:
		case <-d.closeCh:
			return nil
		}
	}
}

// popTaskImpl is the implementation for task popping from the min-heap
func (d *DialQueue) popTaskImpl() *DialTask {
	d.Lock()
	defer d.Unlock()

	if len(d.heap) != 0 {
		// pop the first value and remove it from the heap
		tt := heap.Pop(&d.heap)

		task, ok := tt.(*DialTask)
		if !ok {
			return nil
		}

		return task
	}

	return nil
}

// DeleteTask deletes a task from the dial queue for the specified peer
func (d *DialQueue) DeleteTask(peer peer.ID) {
	if d.closed.Load() {
		return
	}

	d.Lock()
	defer d.Unlock()

	item, ok := d.tasks[peer]
	if ok {
		// negative index for popped element
		if item.index >= 0 {
			heap.Remove(&d.heap, item.index)
		}

		delete(d.tasks, peer)
	}
}

// AddTask adds a new task to the dial queue
func (d *DialQueue) AddTask(
	addrInfo *peer.AddrInfo,
	priority common.DialPriority,
) {
	if d.closed.Load() {
		return
	}

	d.Lock()
	defer d.Unlock()

	task := &DialTask{
		addrInfo: addrInfo,
		priority: uint64(priority),
	}
	d.tasks[addrInfo.ID] = task
	heap.Push(&d.heap, task)

	select {
	case d.updateCh <- struct{}{}:
	default:
	}
}
