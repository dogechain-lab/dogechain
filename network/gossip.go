package network

import (
	"context"
	"reflect"
	"runtime"
	"sync"
	"time"

	"github.com/dogechain-lab/dogechain/helper/common"

	"github.com/hashicorp/go-hclog"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"google.golang.org/protobuf/proto"
)

const (
	// subscribeOutputBufferSize is the size of subscribe output buffer in go-libp2p-pubsub
	// we should have enough capacity of the queue
	// because when queue is full, if the consumer does not read fast enough, new messages are dropped
	subscribeOutputBufferSize = 1024

	_unsubscriptionTimeout = 10 * time.Second
)

// max worker number (min 2 and max 64)
var _workerNum = int(common.Min(common.Max(uint64(runtime.NumCPU()), 2), 64))

type Topic struct {
	logger hclog.Logger

	topic *pubsub.Topic
	typ   reflect.Type

	wg            sync.WaitGroup
	unsubscribeCh chan struct{}
}

func (t *Topic) createObj() proto.Message {
	message, ok := reflect.New(t.typ).Interface().(proto.Message)
	if !ok {
		return nil
	}

	return message
}

func (t *Topic) Publish(obj proto.Message) error {
	data, err := proto.Marshal(obj)
	if err != nil {
		return err
	}

	return t.topic.Publish(context.Background(), data)
}

func (t *Topic) Subscribe(handler func(obj interface{}, from peer.ID)) error {
	sub, err := t.topic.Subscribe(pubsub.WithBufferSize(subscribeOutputBufferSize))
	if err != nil {
		return err
	}

	go t.readLoop(sub, handler)

	return nil
}

func (t *Topic) Close() error {
	close(t.unsubscribeCh)
	t.wg.Wait()

	return t.topic.Close()
}

func (t *Topic) readLoop(sub *pubsub.Subscription, handler func(obj interface{}, from peer.ID)) {
	type task struct {
		Message proto.Message
		From    peer.ID
	}

	// wait group for better close?
	t.wg.Add(1)
	// work queue for less goroutine allocation
	workqueue := make(chan *task, _workerNum*4)
	// cancel context
	ctx, cancel := context.WithCancel(context.Background())

	// wait for the event
	go func() {
		<-t.unsubscribeCh

		// send cancel timeout
		timeoutCtx, timeoutCancel := context.WithTimeout(ctx, _unsubscriptionTimeout)

		go func() {
			sub.Cancel()
			// cancelTimeoutFn() is idempotent, so it's safe to call it multiple times
			// https://stackoverflow.com/questions/59858033/is-cancel-so-mandatory-for-context
			timeoutCancel()
		}()

		// wait for completion or timeout
		<-timeoutCtx.Done()
		timeoutCancel()

		cancel()
		close(workqueue)
		t.wg.Done()
	}()

	for i := 0; i < _workerNum; i++ {
		go func() {
			for {
				task, ok := <-workqueue
				if !ok {
					return
				}

				handler(task.Message, task.From)
			}
		}()
	}

	for {
		select {
		case <-ctx.Done():
			// return when context cancel
			return

		default:
			msg, err := sub.Next(ctx)
			if err != nil {
				t.logger.Error("failed to get topic", "err", err)

				continue
			}

			obj := t.createObj()
			if err := proto.Unmarshal(msg.Data, obj); err != nil {
				t.logger.Error("failed to unmarshal topic", "err", err, "peer", msg.GetFrom())

				continue
			}

			workqueue <- &task{
				Message: obj,
				From:    msg.GetFrom(),
			}
		}
	}
}

func (s *Server) NewTopic(protoID string, obj proto.Message) (*Topic, error) {
	topic, err := s.ps.Join(protoID)
	if err != nil {
		return nil, err
	}

	tt := &Topic{
		logger: s.logger.Named(protoID),

		topic: topic,
		typ:   reflect.TypeOf(obj).Elem(),

		unsubscribeCh: make(chan struct{}),
	}

	return tt, nil
}
