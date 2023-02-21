package network

import (
	"github.com/libp2p/go-libp2p/core/peer"
)

type bootnodesWrapper struct {
	// bootnodeArr is the array that contains all the bootnode addresses
	bootnodeArr []*peer.AddrInfo

	// bootnodesMap is a map used for quick bootnode lookup
	bootnodesMap map[peer.ID]*peer.AddrInfo
}

// isBootnode checks if the node ID belongs to a set bootnode
func (bw *bootnodesWrapper) isBootnode(nodeID peer.ID) bool {
	_, ok := bw.bootnodesMap[nodeID]

	return ok
}

// getBootnodes gets all the bootnodes
func (bw *bootnodesWrapper) getBootnodes() []*peer.AddrInfo {
	return bw.bootnodeArr
}
