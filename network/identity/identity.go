package identity

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dogechain-lab/dogechain/helper/telemetry"
	"github.com/dogechain-lab/dogechain/network/client"
	"github.com/dogechain-lab/dogechain/network/event"
	"github.com/dogechain-lab/dogechain/network/proto"

	"github.com/hashicorp/go-hclog"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

const peerIDMetaString = "peerID"

var (
	ErrInvalidChainID   = errors.New("invalid chain ID")
	ErrNoAvailableSlots = errors.New("no available Slots")
	ErrSelfConnection   = errors.New("self connection")
)

// networkingServer defines the base communication interface between
// any networking server implementation and the IdentityService
type networkingServer interface {
	// PROTOCOL MANIPULATION //

	// NewIdentityClient returns an identity gRPC client connection
	NewIdentityClient(ctx context.Context, peerID peer.ID) (client.IdentityClient, error)

	// PEER MANIPULATION //

	// DisconnectFromPeer attempts to disconnect from the specified peer
	DisconnectFromPeer(peerID peer.ID, reason string)

	// AddPeer adds a peer to the networking server's peer store
	AddPeer(id peer.ID, direction network.Direction)

	// UpdatePendingConnCount updates the pendingPeerConnections connection count for the direction [Thread safe]
	UpdatePendingConnCount(delta int64, direction network.Direction)

	// EmitEvent emits the specified peer event on the base networking server
	EmitEvent(ctx context.Context, event *event.PeerEvent)

	// CONNECTION INFORMATION //

	// HasFreeConnectionSlot checks if there are available outbound connection slots [Thread safe]
	HasFreeConnectionSlot(direction network.Direction) bool

	// GetTracer returns the base networking server's tracer
	GetTracer() telemetry.Tracer
}

// IdentityService is a networking service used to handle peer handshaking.
// It acts as a gatekeeper to peer connectivity
type IdentityService struct {
	proto.UnimplementedIdentityServer

	pendingPeerConnections map[peer.ID]struct{} // Map that keeps track of the pending status of peers; peerID -> bool
	pendingCountMux        sync.RWMutex         // Mutex for the pendingPeerConnections map

	logger hclog.Logger     // The IdentityService logger
	tracer telemetry.Tracer // tracer for the IdentityService

	baseServer networkingServer // The interface towards the base networking server

	chainID int64   // The chain ID of the network
	hostID  peer.ID // The base networking server's host peer ID
}

// NewIdentityService returns a new instance of the IdentityService
func NewIdentityService(
	server networkingServer,
	logger hclog.Logger,
	chainID int64,
	hostID peer.ID,
) *IdentityService {
	return &IdentityService{
		logger:                 logger.Named("identity"),
		tracer:                 server.GetTracer().GetTraceProvider().NewTracer("identity"),
		baseServer:             server,
		chainID:                chainID,
		hostID:                 hostID,
		pendingPeerConnections: make(map[peer.ID]struct{}),
	}
}

func (i *IdentityService) GetNotifyBundle() *network.NotifyBundle {
	return &network.NotifyBundle{
		ConnectedF: func(net network.Network, conn network.Conn) {
			tracer := i.baseServer.GetTracer()

			span := tracer.Start("identity.ConnectedF")
			defer span.End()

			peerID := conn.RemotePeer()
			direction := conn.Stat().Direction

			i.logger.Debug("new conn", "peer", peerID, "direction", direction)

			if i.HasPendingStatus(peerID) {
				// handshake has already started
				span.SetStatus(telemetry.Unset, "already pending")

				return
			}

			// get write lock
			i.pendingCountMux.Lock()
			defer i.pendingCountMux.Unlock()

			// double check
			{
				_, ok := i.pendingPeerConnections[peerID]
				if ok {
					return
				}
			}

			if !i.baseServer.HasFreeConnectionSlot(direction) {
				i.disconnectFromPeer(peerID, ErrNoAvailableSlots.Error())
				span.SetStatus(telemetry.Error, ErrNoAvailableSlots.Error())

				return
			}

			i.addPendingStatus(peerID, direction)
			span.AddEvent("pending status added", map[string]interface{}{
				"peer":      peerID,
				"direction": direction,
			})

			spanCtx := span.SpanContext()

			go func() {
				span := tracer.StartWithParent(spanCtx, "identity.handleConnected")
				defer span.End()

				connectEvent := &event.PeerEvent{
					PeerID:      peerID,
					Type:        event.PeerDialCompleted,
					SpanContext: span.SpanContext(),
				}

				span.SetAttributes(map[string]interface{}{
					"peer":      peerID,
					"direction": direction,
				})

				if err := i.handleConnected(peerID, conn.Stat().Direction); err != nil {
					i.logger.Debug("identity check failed, disconnect peer", "peer", peerID)

					// Close the connection to the peer
					i.disconnectFromPeer(peerID, err.Error())

					i.logger.Debug("send PeerFailedToConnect event", "peer", peerID)

					span.RecordError(err)
					span.SetStatus(telemetry.Error, "identity check failed")

					connectEvent.Type = event.PeerFailedToConnect
				}

				i.removePendingStatus(peerID, direction)

				// Emit an adequate event
				i.baseServer.EmitEvent(span.Context(), connectEvent)
			}()
		},
	}
}

// HasPendingStatus checks if a peer is pending handshake [Thread safe]
func (i *IdentityService) HasPendingStatus(id peer.ID) bool {
	i.pendingCountMux.RLock()
	defer i.pendingCountMux.RUnlock()

	_, ok := i.pendingPeerConnections[id]

	return ok
}

// removePendingStatus removes the pending status from a peer,
// and updates adequate counter information  [Thread safe]
func (i *IdentityService) removePendingStatus(peerID peer.ID, direction network.Direction) {
	i.pendingCountMux.Lock()
	defer i.pendingCountMux.Unlock()

	if _, loaded := i.pendingPeerConnections[peerID]; loaded {
		i.baseServer.UpdatePendingConnCount(-1, direction)

		delete(i.pendingPeerConnections, peerID)
	}
}

// addPendingStatus adds the pending status to a peer,
// and updates adequate counter information
func (i *IdentityService) addPendingStatus(peerID peer.ID, direction network.Direction) {
	i.pendingPeerConnections[peerID] = struct{}{}
	i.baseServer.UpdatePendingConnCount(1, direction)
}

// disconnectFromPeer disconnects from the specified peer
func (i *IdentityService) disconnectFromPeer(peerID peer.ID, reason string) {
	i.baseServer.DisconnectFromPeer(peerID, reason)
}

// handleConnected handles new network connections (handshakes)
func (i *IdentityService) handleConnected(peerID peer.ID, direction network.Direction) error {
	i.logger.Debug("handling new connection", "peer", peerID, "direction", direction)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	// don't save this grpc client object
	// this is a one time use stream
	clt, clientErr := i.baseServer.NewIdentityClient(ctx, peerID)
	if clientErr != nil {
		return fmt.Errorf(
			"unable to create new identity client connection, %w",
			clientErr,
		)
	}

	defer func() {
		err := clt.Close()
		if err != nil {
			i.logger.Error("error closing identity client connection", "error", err)
		}
	}()

	// self peer ID
	selfPeerID := i.hostID.Pretty()

	// Construct the response status
	status := i.constructStatus(peerID)

	// Initiate the handshake
	i.logger.Debug("send hello", "peer", peerID)

	resp, err := clt.Hello(ctx, status)
	if err != nil {
		return err
	}

	// Validate that the peers are working on the same chain
	if status.Chain != resp.Chain {
		return ErrInvalidChainID
	}

	if selfPeerID == resp.Metadata[peerIDMetaString] {
		return ErrSelfConnection
	}

	i.baseServer.AddPeer(peerID, direction)

	return nil
}

// Hello is the initial message that bundles peer information
// on first contact
func (i *IdentityService) Hello(_ context.Context, req *proto.Status) (*proto.Status, error) {
	// The peerID is the other node's peerID
	// as this method is invoking a call such as "Hello, <peerID>!"
	peerID, err := peer.Decode(req.Metadata[peerIDMetaString])
	if err != nil {
		return nil, err
	}

	return i.constructStatus(peerID), nil
}

// constructStatus constructs a status response of the current node
func (i *IdentityService) constructStatus(peerID peer.ID) *proto.Status {
	// deprecated TemporaryDial
	return &proto.Status{
		Metadata: map[string]string{
			peerIDMetaString: i.hostID.Pretty(),
		},
		Chain:         i.chainID,
		TemporaryDial: false,
	}
}
