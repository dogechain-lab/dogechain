package jsonrpc

import "strconv"

// networkStore provides methods needed for Net endpoint
type networkStore interface {
	PeerCount() int64
}

// Net is the net jsonrpc endpoint
type Net struct {
	store   networkStore
	chainID uint64

	metrics *Metrics
}

// Version returns the current network id
func (n *Net) Version() (interface{}, error) {
	n.metrics.NetAPICounterInc(NetVersionLabel)

	return strconv.FormatUint(n.chainID, 10), nil
}

// Listening returns true if client is actively listening for network connections
func (n *Net) Listening() (interface{}, error) {
	n.metrics.NetAPICounterInc(NetListeningLabel)

	return true, nil
}

// PeerCount returns number of peers currently connected to the client
func (n *Net) PeerCount() (interface{}, error) {
	n.metrics.NetAPICounterInc(NetPeerCountLabel)

	peers := n.store.PeerCount()

	return strconv.FormatInt(peers, 10), nil
}
