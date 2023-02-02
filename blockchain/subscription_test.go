package blockchain

import (
	"context"
	"testing"
	"time"

	"github.com/dogechain-lab/dogechain/types"
	"github.com/stretchr/testify/assert"
)

func TestSubscription(t *testing.T) {
	t.Parallel()

	var (
		e              = newEventStream(context.Background())
		sub            = e.subscribe()
		caughtEventNum = uint64(0)
		event          = &Event{
			NewChain: []*types.Header{
				{
					Number: 100,
				},
			},
		}
	)

	result := make(chan struct{}, 1)

	t.Cleanup(func() {
		close(result)
		e.Close()
	})

	go func() {
		// Wait for the event to be parsed
		evnt := <-sub.GetEvent()
		caughtEventNum = evnt.NewChain[0].Number
		result <- struct{}{}
	}()

	// Send the event to the channel
	e.push(event)

	select {
	case <-result:
	case <-time.After(5 * time.Second):
		t.Failed()
	}

	assert.Equal(t, event.NewChain[0].Number, caughtEventNum)
}
