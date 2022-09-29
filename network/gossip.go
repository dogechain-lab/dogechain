package network

import (
	"context"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/hashicorp/go-hclog"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"google.golang.org/protobuf/proto"
)

const (
	// subscribeOutputBufferSize is the size of subscribe output buffer in go-libp2p-pubsub
	// we should have enough capacity of the queue
	// because when queue is full, if the consumer does not read fast enough, new messages are dropped
	subscribeOutputBufferSize = 1024
)

type Topic struct {
	logger hclog.Logger

	topic *pubsub.Topic
	typ   reflect.Type

	wg            *sync.WaitGroup
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

func (t *Topic) Subscribe(handler func(obj interface{})) error {
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

func (t *Topic) readLoop(sub *pubsub.Subscription, handler func(obj interface{})) {
	ctx, cancelFn := context.WithCancel(context.Background())

	var unsubscribe int32 = 0

	workqueue := make(chan proto.Message, runtime.NumCPU())

	t.wg.Add(1)

	go func() {
		<-t.unsubscribeCh
		atomic.StoreInt32(&unsubscribe, 1)

		close(workqueue)

		cancelFn()
		sub.Cancel()
		t.wg.Done()
	}()

	for i := 0; i < runtime.NumCPU(); i++ {
		go func() {
			for {
				obj, ok := <-workqueue
				if !ok {
					return
				}

				handler(obj)
			}
		}()
	}

	for atomic.LoadInt32(&unsubscribe) == 0 {
		msg, err := sub.Next(ctx)
		if err != nil {
			t.logger.Error("failed to get topic", "err", err)

			continue
		}

		obj := t.createObj()
		if err := proto.Unmarshal(msg.Data, obj); err != nil {
			t.logger.Error("failed to unmarshal topic", "err", err)

			return
		}

		workqueue <- obj
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

		wg:            &sync.WaitGroup{},
		unsubscribeCh: make(chan struct{}),
	}

	return tt, nil
}
