package network

import (
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
)

type staticnodesWrapper struct {
	mux sync.RWMutex

	peers map[peer.ID]*peer.AddrInfo
}

func newStaticnodesWrapper() *staticnodesWrapper {
	return &staticnodesWrapper{
		peers: make(map[peer.ID]*peer.AddrInfo),
	}
}

func (sw *staticnodesWrapper) Len() int {
	sw.mux.RLock()
	defer sw.mux.RUnlock()

	return len(sw.peers)
}

// addStaticnode adds a staticnode to the staticnode list
func (sw *staticnodesWrapper) addStaticnode(addr *peer.AddrInfo) {
	sw.mux.Lock()
	defer sw.mux.Unlock()

	if addr == nil {
		panic("addr is nil")
	}

	sw.peers[addr.ID] = addr
}

func (sw *staticnodesWrapper) rangeAddrs(f func(add *peer.AddrInfo) bool) {
	sw.mux.RLock()
	defer sw.mux.RUnlock()

	for _, addr := range sw.peers {
		if addr != nil {
			f(addr)
		}
	}
}

// isStaticnode checks if the node ID belongs to a set staticnode
func (sw *staticnodesWrapper) isStaticnode(nodeID peer.ID) bool {
	sw.mux.RLock()
	defer sw.mux.RUnlock()

	_, ok := sw.peers[nodeID]

	return ok
}
