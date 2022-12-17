package protocol

import (
	"sort"
	"sync"
	"testing"

	"github.com/libp2p/go-libp2p-core/peer"
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

// ToList return all peers (sort by IsBetter)
func (m *PeerMap) toList() []*NoForkPeer {
	// get all values
	var peers = []*NoForkPeer{}

	m.Range(func(_, val interface{}) bool {
		peer, ok := val.(*NoForkPeer)
		if ok {
			peers = append(peers, peer)
		}

		return true
	})

	return sortNoForkPeers(peers)
}

func sortNoForkPeers(peers []*NoForkPeer) []*NoForkPeer {
	sort.SliceStable(peers, func(p, q int) bool {
		return peers[p].IsBetter(peers[q])
	})

	return peers
}

func peerMapToPeers(peerMap *PeerMap) []*NoForkPeer {
	peers := peerMap.toList()

	sort.Slice(peers, func(i, j int) bool {
		return peers[i].IsBetter(peers[j])
	})

	return peers
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
		},
		{
			name:     "should return null in case of empty map",
			skipList: nil,
			peers:    nil,
		},
		{
			name:     "should return the 2nd best peer if the best peer is in skip list",
			skipList: skipList,
			peers:    peers,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			peerMap := NewPeerMap(test.peers)

			betterPeer := peerMap.BetterPeer(test.skipList, peers[0].Number)

			if test.peers != nil {
				assert.NotNil(t, betterPeer)

				assert.True(
					t,
					betterPeer.Number > peers[0].Number,
				)
			} else {
				assert.Nil(t, betterPeer)
			}
		})
	}
}
