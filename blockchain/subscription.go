package blockchain

import (
	"context"
	"sync"

	"go.uber.org/atomic"
)

// Subscription is the blockchain subscription interface
type Subscription interface {
	GetEvent() *Event

	IsClosed() bool
}

// FOR TESTING PURPOSES //

type MockSubscription struct {
	eventCh chan *Event
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
	return false
}

/////////////////////////

// subscription is the Blockchain event subscription object
type subscription struct {
	// Channel for update information
	// close from eventStream
	updateCh chan *Event

	// context is the context for the event stream
	ctx context.Context

	// contextCancel is the cancel function for the context
	ctxCancel context.CancelFunc

	// closed is a flag that indicates if the subscription is closed
	closed *atomic.Bool
}

// GetEvent returns the event from the subscription (BLOCKING)
func (s *subscription) GetEvent() *Event {
	if s.closed.Load() {
		return nil
	}

	select {
	case <-s.ctx.Done():
		s.closed.Store(true)

		return nil
	case ev, ok := <-s.updateCh:
		if ok {
			return ev
		}
	}

	return nil
}

// IsClosed returns true if the subscription is closed
func (s *subscription) IsClosed() bool {
	return s.closed.Load()
}

type EventType int

const (
	EventHead  EventType = iota // New head event
	EventReorg                  // Chain reorganization event
	EventFork                   // Chain fork event
)

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

	stream := &eventStream{
		ctx:       streamCtx,
		ctxCancel: cancel,
	}

	return stream
}

func (e *eventStream) Close() {
	e.ctxCancel()

	e.lock.Lock()
	defer e.lock.Unlock()

	for _, ch := range e.updateCh {
		close(ch)
	}

	e.updateCh = e.updateCh[0:0]
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

	ch := make(chan *Event, 8)
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
		case <-e.ctx.Done():
			return
		case update <- event:
		default:
		}
	}
}
