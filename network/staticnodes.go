package network

import (
	"sync"

	"github.com/libp2p/go-libp2p-core/peer"
	"go.uber.org/atomic"
)

type staticnodesWrapper struct {
	m sync.Map

	// count add node
	count atomic.Int64
}

func newStaticnodesWrapper() *staticnodesWrapper {
	return &staticnodesWrapper{
		m:     sync.Map{},
		count: atomic.Int64{},
	}
}

func (sw *staticnodesWrapper) Len() int {
	return int(sw.count.Load())
}

// addStaticnode adds a staticnode to the staticnode list
func (sw *staticnodesWrapper) addStaticnode(addr *peer.AddrInfo) {
	if addr == nil {
		panic("addr is nil")
	}

	sw.m.Store(addr.ID, addr)
	sw.count.Inc()
}

func (sw *staticnodesWrapper) rangeAddrs(f func(add *peer.AddrInfo) bool) {
	sw.m.Range(func(_, value interface{}) bool {
		if addr, ok := value.(*peer.AddrInfo); addr != nil && ok {
			return f(addr)
		}

		// continue, skip this value
		return true
	})
}

// isStaticnode checks if the node ID belongs to a set staticnode
func (sw *staticnodesWrapper) isStaticnode(nodeID peer.ID) bool {
	_, ok := sw.m.Load(nodeID)

	return ok
}
