package protocol

import (
	"sort"
	"sync"

	"github.com/libp2p/go-libp2p-core/peer"
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

// ToList return all peers (sort by IsBetter)
func (m *PeerMap) ToList() []*NoForkPeer {
	// get all values
	var peers = []*NoForkPeer{}

	m.Range(func(_, val interface{}) bool {
		peer, ok := val.(*NoForkPeer)
		if ok {
			peers = append(peers, peer)
		}

		return true
	})

	sort.Slice(peers, func(i, j int) bool {
		return peers[i].IsBetter(peers[j])
	})

	return peers
}

// BetterPeer returns higher node
func (m *PeerMap) BetterPeer(skipMap *sync.Map, height uint64) *NoForkPeer {
	var betterPeer *NoForkPeer

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
		peer, ok := value.(*NoForkPeer)
		if !ok || needSkipPeer(skipMap, peer.ID) {
			return true
		}

		if betterPeer == nil && peer.Number > height {
			betterPeer = peer

			return false
		}

		return true
	})

	return betterPeer
}
