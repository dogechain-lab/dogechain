package network

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"runtime"
	"testing"
	"time"

	"go.uber.org/atomic"

	testproto "github.com/dogechain-lab/dogechain/network/proto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/assert"
)

const (
	testGossipTopicName = "msg-pub-sub"
)

func NumSubscribers(srv *Server, topic string) int {
	return len(srv.ps.ListPeers(topic))
}

func WaitForSubscribers(ctx context.Context, srv *Server, topic string, expectedNumPeers int) error {
	for {
		if n := NumSubscribers(srv, topic); n >= expectedNumPeers {
			return nil
		}

		delay := time.NewTimer(100 * time.Millisecond)
		defer delay.Stop()

		select {
		case <-ctx.Done():
			return errors.New("canceled")
		case <-delay.C:
			continue
		}
	}
}

func TestSimpleGossip(t *testing.T) {
	numServers := 10
	sentMessage := fmt.Sprintf("%d", time.Now().Unix())
	servers, createErr := createServers(numServers, nil)
	topicName := fmt.Sprintf(testGossipTopicName+"-%d", time.Now().UnixNano())

	if createErr != nil {
		t.Fatalf("Unable to create servers, %v", createErr)
	}

	messageCh := make(chan *testproto.GenericMessage)

	t.Cleanup(func() {
		close(messageCh)
		closeTestServers(t, servers)
	})

	joinErrors := MeshJoin(servers...)
	if len(joinErrors) != 0 {
		t.Fatalf("Unable to join servers [%d], %v", len(joinErrors), joinErrors)
	}

	serverTopics := make([]*Topic, numServers)

	for i := 0; i < numServers; i++ {
		topic, topicErr := servers[i].NewTopic(topicName, &testproto.GenericMessage{})
		if topicErr != nil {
			t.Fatalf("Unable to create topic, %v", topicErr)
		}

		serverTopics[i] = topic

		if subscribeErr := topic.Subscribe(func(obj interface{}, _ peer.ID) {
			// Everyone should relay they got the message
			genericMessage, ok := obj.(*testproto.GenericMessage)
			if !ok {
				t.Fatalf("invalid type assert")
			}

			messageCh <- genericMessage
		}); subscribeErr != nil {
			t.Fatalf("Unable to subscribe to topic, %v", subscribeErr)
		}
	}

	publisher := servers[0]
	publisherTopic := serverTopics[0]

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if waitErr := WaitForSubscribers(ctx, publisher, topicName, len(servers)-1); waitErr != nil {
		t.Fatalf("Unable to wait for subscribers, %v", waitErr)
	}

	if publishErr := publisherTopic.Publish(
		&testproto.GenericMessage{
			Message: sentMessage,
		}); publishErr != nil {
		t.Fatalf("Unable to publish message, %v", publishErr)
	}

	messagesGossiped := 0

	for {
		delay := time.NewTimer(15 * time.Second)
		defer delay.Stop()

		select {
		case <-delay.C:
			t.Fatalf("Gossip messages not received before timeout")
		case message := <-messageCh:
			if message.Message == sentMessage {
				messagesGossiped++
				if messagesGossiped == len(servers) {
					return
				}
			}
		}
	}
}

func TestTopicBackpressure(t *testing.T) {
	numServers := 3
	sentMessage := fmt.Sprintf("%d", time.Now().Unix())
	topicName := fmt.Sprintf(testGossipTopicName+"-%d", time.Now().UnixNano())
	servers, createErr := createServers(numServers, nil)
	subscribeCloseCh := make(chan struct{})

	subscribeGoroutineCount := &atomic.Int32{}

	if createErr != nil {
		t.Fatalf("Unable to create servers, %v", createErr)
	}

	t.Cleanup(func() {
		close(subscribeCloseCh)
		closeTestServers(t, servers)
	})

	joinErrors := MeshJoin(servers...)
	if len(joinErrors) != 0 {
		t.Fatalf("Unable to join servers [%d], %v", len(joinErrors), joinErrors)
	}

	serverTopics := make([]*Topic, numServers)

	for i := 0; i < numServers; i++ {
		topic, topicErr := servers[i].NewTopic(topicName, &testproto.GenericMessage{})
		if topicErr != nil {
			t.Fatalf("Unable to create topic, %v", topicErr)
		}

		serverTopics[i] = topic

		if subscribeErr := topic.Subscribe(func(obj interface{}, _ peer.ID) {
			subscribeGoroutineCount.Add(1)

			// wait for the channel to close
			<-subscribeCloseCh
		}); subscribeErr != nil {
			t.Fatalf("Unable to subscribe to topic, %v", subscribeErr)
		}
	}

	publisherTopic := serverTopics[0]
	//#nosec G404
	randomSendMessageNum := rand.Intn(100) + runtime.NumCPU()*numServers

	for i := 0; i < randomSendMessageNum; i++ {
		if publishErr := publisherTopic.Publish(
			&testproto.GenericMessage{
				Message: sentMessage,
			}); publishErr != nil {
			t.Fatalf("Unable to publish message, %v", publishErr)
		}
	}

	// subscribe handler backpressure
	assert.LessOrEqual(t, subscribeGoroutineCount.Load(), int32(runtime.NumCPU()*numServers))
}

func TestTopicClose(t *testing.T) {
	numServers := 5
	sentMessage := fmt.Sprintf("%d", time.Now().Unix())
	topicName := fmt.Sprintf(testGossipTopicName+"-%d", time.Now().UnixNano())
	servers, createErr := createServers(numServers, nil)
	subscribeCloseCh := make(chan struct{})

	subscribeCount := make([]*atomic.Int32, numServers)
	for i := 0; i < numServers; i++ {
		subscribeCount[i] = &atomic.Int32{}
	}

	if createErr != nil {
		t.Fatalf("Unable to create servers, %v", createErr)
	}

	t.Cleanup(func() {
		close(subscribeCloseCh)
		closeTestServers(t, servers)
	})

	joinErrors := MeshJoin(servers...)
	if len(joinErrors) != 0 {
		t.Fatalf("Unable to join servers [%d], %v", len(joinErrors), joinErrors)
	}

	serverTopics := make([]*Topic, numServers)

	for i := 0; i < numServers; i++ {
		var count = subscribeCount[i]

		topic, topicErr := servers[i].NewTopic(topicName, &testproto.GenericMessage{})
		if topicErr != nil {
			t.Fatalf("Unable to create topic, %v", topicErr)
		}

		serverTopics[i] = topic

		if subscribeErr := topic.Subscribe(func(obj interface{}, _ peer.ID) {
			count.Add(1)

			// wait for the channel to close
			<-subscribeCloseCh
		}); subscribeErr != nil {
			t.Fatalf("Unable to subscribe to topic, %v", subscribeErr)
		}
	}

	//#nosec G404
	randomSeed := rand.Int()

	publisherIndex := randomSeed % numServers
	publisherTopic := serverTopics[publisherIndex]

	closeTopicServerIndex := (randomSeed + 1) % numServers
	closerTopic := serverTopics[closeTopicServerIndex]

	closerTopic.Close()

	//#nosec G404
	randomSendMessageNum := rand.Intn(100) + 100

	for i := 0; i < randomSendMessageNum; i++ {
		if publishErr := publisherTopic.Publish(
			&testproto.GenericMessage{
				Message: sentMessage,
			}); publishErr != nil {
			t.Fatalf("Unable to publish message, %v", publishErr)
		}
	}

	// close topic server should not receive any messages
	assert.Equal(t, subscribeCount[closeTopicServerIndex].Load(), int32(0))
}
