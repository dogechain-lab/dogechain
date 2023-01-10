package network

import (
	"sync"

	"github.com/libp2p/go-libp2p-core/peer"
)

type staticnodesWrapper struct {
	mux sync.RWMutex

	// staticnodeArr is the array that contains all the staticnode addresses
	staticnodesArr []*peer.AddrInfo

	// staticnodesMap is a map used for quick staticnode lookup
	staticnodesMap map[peer.ID]*peer.AddrInfo
}

// addStaticnode adds a staticnode to the staticnode list
func (sw *staticnodesWrapper) addStaticnode(addr *peer.AddrInfo) {
	sw.mux.Lock()
	defer sw.mux.Unlock()

	sw.staticnodesArr = append(sw.staticnodesArr, addr)
	sw.staticnodesMap[addr.ID] = addr
}

func (sw *staticnodesWrapper) rangeAddrs(f func(add *peer.AddrInfo) bool) {
	sw.mux.RLock()
	defer sw.mux.RUnlock()

	for _, addr := range sw.staticnodesArr {
		if !f(addr) {
			break
		}
	}
}

// isStaticnode checks if the node ID belongs to a set staticnode
func (sw *staticnodesWrapper) isStaticnode(nodeID peer.ID) bool {
	sw.mux.RLock()
	defer sw.mux.RUnlock()

	_, ok := sw.staticnodesMap[nodeID]

	return ok
}
