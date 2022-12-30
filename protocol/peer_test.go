package protocol

import (
	"sort"
	"sync"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
)

var (
	peers = []*NoForkPeer{
		{
			ID:     peer.ID("A"),
			Number: 10,
		},
		{
			ID:     peer.ID("B"),
			Number: 30,
		},
		{
			ID:     peer.ID("C"),
			Number: 20,
		},
	}
)

func cloneNoForkPeers(peers []*NoForkPeer) []*NoForkPeer {
	clone := make([]*NoForkPeer, len(peers))

	for idx, p := range peers {
		clone[idx] = &NoForkPeer{
			ID:     p.ID,
			Number: p.Number,
		}
	}

	return clone
}

func sortNoForkPeers(peers []*NoForkPeer) []*NoForkPeer {
	sort.SliceStable(peers, func(p, q int) bool {
		return peers[p].Number > peers[q].Number
	})

	return peers
}

func peerMapToPeers(peerMap *PeerMap) []*NoForkPeer {
	res := make([]*NoForkPeer, 0)

	for {
		bestPeer := peerMap.BestPeer(nil)
		if bestPeer == nil {
			break
		}

		res = append(res, bestPeer)

		peerMap.Remove(bestPeer.ID)
	}

	return res
}

func TestConstructor(t *testing.T) {
	t.Parallel()

	peers := peers

	peerMap := NewPeerMap(peers)

	expected := sortNoForkPeers(
		cloneNoForkPeers(peers),
	)

	actual := peerMapToPeers(peerMap)

	assert.Equal(
		t,
		expected,
		actual,
	)
}

func TestPutPeer(t *testing.T) {
	t.Parallel()

	initialPeers := peers[:1]
	peers := peers[1:]

	peerMap := NewPeerMap(initialPeers)

	peerMap.Put(peers...)

	expected := sortNoForkPeers(
		cloneNoForkPeers(append(initialPeers, peers...)),
	)

	actual := peerMapToPeers(peerMap)

	assert.Equal(
		t,
		expected,
		actual,
	)
}

func TestBestPeer(t *testing.T) {
	t.Parallel()

	skipList := new(sync.Map)
	skipList.Store(peer.ID("C"), true)

	tests := []struct {
		name     string
		skipList *sync.Map
		peers    []*NoForkPeer
		result   *NoForkPeer
	}{
		{
			name:     "should return best peer",
			skipList: nil,
			peers:    peers,
			result:   peers[1],
		},
		{
			name:     "should return null in case of empty map",
			skipList: nil,
			peers:    nil,
			result:   nil,
		},
		{
			name:     "should return the 2nd best peer if the best peer is in skip list",
			skipList: skipList,
			peers:    peers,
			result:   peers[1],
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			peerMap := NewPeerMap(test.peers)

			bestPeer := peerMap.BestPeer(test.skipList)

			assert.Equal(t, test.result, bestPeer)
		})
	}
}
