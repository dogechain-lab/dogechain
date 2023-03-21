package network

import (
	"context"

	"github.com/dogechain-lab/dogechain/helper/telemetry"
	"github.com/dogechain-lab/dogechain/network/client"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

// PeerConnInfo holds the connection information about the peer
type PeerConnInfo struct {
	Info peer.AddrInfo

	connDirections map[network.Direction]bool
	protocolClient map[string]client.GrpcClientCloser
}

// addConnDirection adds a connection direction
func (pci *PeerConnInfo) addConnDirection(direction network.Direction) {
	pci.connDirections[direction] = true
}

// removeConnDirection adds a connection direction
func (pci *PeerConnInfo) removeConnDirection(direction network.Direction) {
	pci.connDirections[direction] = false
}

// existsConnDirection returns the connection direction
func (pci *PeerConnInfo) existsConnDirection(direction network.Direction) bool {
	exist, ok := pci.connDirections[direction]
	if !ok {
		return false
	}

	return exist
}

func (pci *PeerConnInfo) noConnectionAvailable() bool {
	// if all directions are false, return false
	for _, v := range pci.connDirections {
		if v {
			return false
		}
	}

	return true
}

// addProtocolClient adds a protocol stream
func (pci *PeerConnInfo) addProtocolClient(protocol string, stream client.GrpcClientCloser) {
	pci.protocolClient[protocol] = stream
}

// cleanProtocolStreams clean and closes all protocol stream
func (pci *PeerConnInfo) cleanProtocolStreams(ctx context.Context, trace telemetry.Tracer) []error {
	errs := []error{}
	span := trace.StartWithContext(ctx, "PeerConnInfo.cleanProtocolStreams")

	defer span.End()

	for protocolName, clt := range pci.protocolClient {
		if clt != nil {
			err := clt.Close()
			if err != nil {
				span.AddEvent(
					"close_error",
					map[string]interface{}{
						"protocol": protocolName,
						"error":    err.Error(),
					})
			}

			errs = append(errs, err)
		}
	}

	pci.protocolClient = make(map[string]client.GrpcClientCloser)

	return errs
}

// getProtocolClient fetches the protocol stream, if any
func (pci *PeerConnInfo) getProtocolClient(protocol string) client.GrpcClientCloser {
	return pci.protocolClient[protocol]
}
