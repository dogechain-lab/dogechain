package blockchain

import (
	"sync"
	"testing"
	"time"

	"github.com/dogechain-lab/dogechain/types"
	"github.com/stretchr/testify/assert"
)

func TestSubscription(t *testing.T) {
	t.Parallel()

	var (
		e              = &eventStream{}
		sub            = e.subscribe()
		caughtEventNum = uint64(0)
		event          = &Event{
			NewChain: []*types.Header{
				{
					Number: 100,
				},
			},
		}

		wg sync.WaitGroup
	)

	defer sub.Close()

	updateCh := sub.GetEventCh()

	wg.Add(1)

	go func() {
		defer wg.Done()

		select {
		case ev := <-updateCh:
			caughtEventNum = ev.NewChain[0].Number
		case <-time.After(5 * time.Second):
		}
	}()

	// Send the event to the channel
	e.push(event)

	// Wait for the event to be parsed
	wg.Wait()

	assert.Equal(t, event.NewChain[0].Number, caughtEventNum)
}

func TestSubscriptionSlowConsumer(t *testing.T) {
	e := &eventStream{}

	e.push(&Event{
		NewChain: []*types.Header{
			{Number: 0},
		},
	})

	sub := e.subscribe()

	// send multiple events
	for i := 1; i < 10; i++ {
		e.push(&Event{
			NewChain: []*types.Header{
				{Number: uint64(i)},
			},
		})
	}

	// consume events now
	for i := 1; i < 10; i++ {
		evnt := sub.GetEvent()
		if evnt.NewChain[0].Number != uint64(i) {
			t.Fatal("bad")
		}
	}
}
