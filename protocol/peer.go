package protocol

import (
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
)

type NoForkPeer struct {
	// identifier
	ID peer.ID
	// peer's latest block number
	Number uint64

	// distance is DHT distance, but not good enough for peer selecting
	// // peer's distance
	// Distance *big.Int
}

func (p *NoForkPeer) IsBetter(t *NoForkPeer) bool {
	return p.Number > t.Number
}

type PeerMap struct {
	sync.Map
}

func NewPeerMap(peers []*NoForkPeer) *PeerMap {
	peerMap := new(PeerMap)

	peerMap.Put(peers...)

	return peerMap
}

func (m *PeerMap) Exists(peerID peer.ID) bool {
	_, exists := m.Load(peerID.String())

	return exists
}

func (m *PeerMap) Put(peers ...*NoForkPeer) {
	for _, peer := range peers {
		m.Store(peer.ID.String(), peer)
	}
}

// Remove removes a peer from heap if it exists
func (m *PeerMap) Remove(peerID peer.ID) {
	m.Delete(peerID.String())
}

// BestPeer returns the top of heap
func (m *PeerMap) BestPeer(skipMap *sync.Map) *NoForkPeer {
	var bestPeer *NoForkPeer

	needSkipPeer := func(skipMap *sync.Map, id peer.ID) bool {
		if skipMap == nil {
			return false
		}

		v, exists := skipMap.Load(id)
		if !exists {
			return false
		}

		skip, ok := v.(bool)

		return ok && skip
	}

	m.Range(func(key, value interface{}) bool {
		peer, _ := value.(*NoForkPeer)

		if needSkipPeer(skipMap, peer.ID) {
			return true
		}

		if bestPeer == nil || peer.IsBetter(bestPeer) {
			bestPeer = peer
		}

		return true
	})

	return bestPeer
}
