package blockchain

import (
	"context"
	"math/big"
	"sync"

	"github.com/dogechain-lab/dogechain/types"
	"go.uber.org/atomic"
)

// Subscription is the blockchain subscription interface
type Subscription interface {
	GetEvent() *Event

	IsClosed() bool
	Close()
}

// FOR TESTING PURPOSES //

type MockSubscription struct {
	eventCh  chan *Event
	isClosed atomic.Bool
}

func NewMockSubscription() *MockSubscription {
	return &MockSubscription{eventCh: make(chan *Event)}
}

func (m *MockSubscription) Push(e *Event) {
	m.eventCh <- e
}

func (m *MockSubscription) GetEvent() *Event {
	evnt := <-m.eventCh

	return evnt
}

func (m *MockSubscription) IsClosed() bool {
	return m.isClosed.Load()
}

func (m *MockSubscription) Close() {
	if m.isClosed.CAS(false, true) {
		close(m.eventCh)
	}
}

/////////////////////////

// subscription is the Blockchain event subscription object
type subscription struct {
	updateCh chan *Event // Channel for update information

	// context is the context for the event stream
	ctx context.Context

	// contextCancel is the cancel function for the context
	ctxCancel context.CancelFunc

	// closed is a flag that indicates if the subscription is closed
	closed *atomic.Bool
}

// GetEvent returns the event from the subscription (BLOCKING)
func (s *subscription) GetEvent() *Event {
	for {
		// Wait for an update
		select {
		case <-s.ctx.Done():
			return nil
		case ev := <-s.updateCh:
			return ev
		}
	}
}

// IsClosed returns true if the subscription is closed
func (s *subscription) IsClosed() bool {
	return s.closed.Load()
}

// Close closes the subscription
func (s *subscription) Close() {
	if s.closed.CAS(false, true) {
		s.ctxCancel()
	}
}

type EventType int

const (
	EventHead  EventType = iota // New head event
	EventReorg                  // Chain reorganization event
	EventFork                   // Chain fork event
)

// Event is the blockchain event that gets passed to the listeners
type Event struct {
	// Old chain (removed headers) if there was a reorg
	OldChain []*types.Header

	// New part of the chain (or a fork)
	NewChain []*types.Header

	// Difficulty is the new difficulty created with this event
	Difficulty *big.Int

	// Type is the type of event
	Type EventType

	// Source is the source that generated the blocks for the event
	// right now it can be either the Sealer or the Syncer
	Source string
}

// Header returns the latest block header for the event
func (e *Event) Header() *types.Header {
	return e.NewChain[len(e.NewChain)-1]
}

// SetDifficulty sets the event difficulty
func (e *Event) SetDifficulty(b *big.Int) {
	e.Difficulty = new(big.Int).Set(b)
}

// AddNewHeader appends a header to the event's NewChain array
func (e *Event) AddNewHeader(newHeader *types.Header) {
	header := newHeader.Copy()

	if e.NewChain == nil {
		// Array doesn't exist yet, create it
		e.NewChain = []*types.Header{}
	}

	e.NewChain = append(e.NewChain, header)
}

// AddOldHeader appends a header to the event's OldChain array
func (e *Event) AddOldHeader(oldHeader *types.Header) {
	header := oldHeader.Copy()

	if e.OldChain == nil {
		// Array doesn't exist yet, create it
		e.OldChain = []*types.Header{}
	}

	e.OldChain = append(e.OldChain, header)
}

// SubscribeEvents returns a blockchain event subscription
func (b *Blockchain) SubscribeEvents() Subscription {
	return b.stream.subscribe()
}

// eventStream is the structure that contains the event list,
// as well as the update channel which it uses to notify of updates
type eventStream struct {
	lock sync.Mutex

	// context is the context for the event stream
	ctx context.Context

	// contextCancel is the cancel function for the context
	ctxCancel context.CancelFunc

	// channel to notify updates
	updateCh []chan *Event
}

func newEventStream(ctx context.Context) *eventStream {
	streamCtx, cancel := context.WithCancel(ctx)

	return &eventStream{
		ctx:       streamCtx,
		ctxCancel: cancel,
	}
}

func (e *eventStream) Close() {
	e.ctxCancel()
}

// subscribe Creates a new blockchain event subscription
func (e *eventStream) subscribe() *subscription {
	subCtx, cancel := context.WithCancel(e.ctx)

	return &subscription{
		updateCh:  e.newUpdateCh(),
		ctx:       subCtx,
		ctxCancel: cancel,
		closed:    atomic.NewBool(false),
	}
}

// newUpdateCh returns the event update channel
func (e *eventStream) newUpdateCh() chan *Event {
	e.lock.Lock()
	defer e.lock.Unlock()

	ch := make(chan *Event, 1)
	e.updateCh = append(e.updateCh, ch)

	return ch
}

// push adds a new Event, and notifies listeners
func (e *eventStream) push(event *Event) {
	e.lock.Lock()
	defer e.lock.Unlock()

	// Notify the listeners
	for _, update := range e.updateCh {
		select {
		case update <- event:
		default:
		}
	}
}
