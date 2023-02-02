package blockchain

import (
	"context"
	"sync"

	"go.uber.org/atomic"
)

// Subscription is the blockchain subscription interface
type Subscription interface {
	GetEvent() <-chan *Event

	IsClosed() bool
	Unsubscribe()
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

func (m *MockSubscription) GetEvent() <-chan *Event {
	return m.eventCh
}

func (m *MockSubscription) IsClosed() bool {
	return m.isClosed.Load()
}

func (m *MockSubscription) Unsubscribe() {
	m.isClosed.Store(true)
}

/////////////////////////

// subscription is the Blockchain event subscription object
type subscription struct {
	// Channel for update information
	// close from eventStream
	updateCh chan *Event

	// closed is a flag that indicates if the subscription is closed
	closed *atomic.Bool
}

// GetEvent returns the event from the subscription (BLOCKING)
func (s *subscription) GetEvent() <-chan *Event {
	return s.updateCh
}

// IsClosed returns true if the subscription is closed
func (s *subscription) IsClosed() bool {
	return s.closed.Load()
}

// Unsubscribe closes the subscription
func (s *subscription) Unsubscribe() {
	// don't close updateCh, it's closed from eventStream
	s.closed.CAS(false, true)
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
	lock sync.RWMutex

	// context is the context for the event stream
	ctx context.Context

	// contextCancel is the cancel function for the context
	ctxCancel context.CancelFunc

	// channel to notify updates
	updateSubCh map[*subscription]chan *Event

	// channel to notify new subscriptions
	subCh chan *subscription

	// event channel
	eventCh chan *Event

	isClosed *atomic.Bool
}

func newEventStream(ctx context.Context) *eventStream {
	streamCtx, cancel := context.WithCancel(ctx)

	stream := &eventStream{
		ctx:         streamCtx,
		ctxCancel:   cancel,
		updateSubCh: make(map[*subscription]chan *Event),
		// use buffered channel need fix unit test
		subCh:    make(chan *subscription),
		eventCh:  make(chan *Event),
		isClosed: atomic.NewBool(false),
	}

	go stream.run()

	return stream
}

func (e *eventStream) run() {
	defer func() {
		close(e.subCh)
		close(e.eventCh)
	}()

	closeSub := make([]*subscription, 0)

	for {
		select {
		case <-e.ctx.Done():
			return
		case newSub := <-e.subCh:
			e.lock.Lock()

			// add the new subscription to the list
			_, ok := e.updateSubCh[newSub]
			if !ok {
				e.updateSubCh[newSub] = newSub.updateCh
			}

			e.lock.Unlock()
		case event := <-e.eventCh:
			e.lock.RLock()

			// Notify the listeners
			for sub, updateCh := range e.updateSubCh {
				if sub.IsClosed() {
					closeSub = append(closeSub, sub)

					continue
				}

				select {
				case <-e.ctx.Done():
					return
				case updateCh <- event:
				default:
				}
			}

			e.lock.RUnlock()

			e.lock.Lock()

			// clear closed subscriptions
			for _, sub := range closeSub {
				delete(e.updateSubCh, sub)
				close(sub.updateCh)
			}

			e.lock.Unlock()

			closeSub = closeSub[:0]
		}
	}
}

func (e *eventStream) Close() {
	if !e.isClosed.CAS(false, true) {
		return
	}

	e.ctxCancel()

	e.lock.Lock()
	defer e.lock.Unlock()

	for update := range e.updateSubCh {
		close(update.updateCh)
		delete(e.updateSubCh, update)
	}
}

// subscribe Creates a new blockchain event subscription
func (e *eventStream) subscribe() *subscription {
	if e.isClosed.Load() {
		return nil
	}

	// check if the context is done
	select {
	case <-e.ctx.Done():
		return nil
	default:
	}

	sub := &subscription{
		updateCh: make(chan *Event, 8),
		closed:   atomic.NewBool(false),
	}

	select {
	case <-e.ctx.Done():
		return nil
	case e.subCh <- sub:
	}

	return sub
}

// push adds a new Event, and notifies listeners
func (e *eventStream) push(event *Event) {
	e.eventCh <- event
}
