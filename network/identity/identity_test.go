package identity

import (
	"context"
	"testing"

	cmap "github.com/dogechain-lab/dogechain/helper/concurrentmap"
	"github.com/dogechain-lab/dogechain/network/proto"
	networkTesting "github.com/dogechain-lab/dogechain/network/testing"
	"github.com/hashicorp/go-hclog"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

// newIdentityService creates a new identity service instance
// with mock-able backends
func newIdentityService(
	networkingServerCallback func(server *networkTesting.MockNetworkingServer),
) *IdentityService {
	baseServer := networkTesting.NewMockNetworkingServer()

	if networkingServerCallback != nil {
		networkingServerCallback(baseServer)
	}

	return &IdentityService{
		baseServer:             baseServer,
		logger:                 hclog.NewNullLogger(),
		pendingPeerConnections: cmap.NewConcurrentMap(),
	}
}

// TestSelfConnected tests connect self to self
func TestSelfConnected(t *testing.T) {
	peersArray := make([]peer.ID, 0)

	identityService := newIdentityService(
		// Set the relevant hook responses from the mock server
		func(server *networkTesting.MockNetworkingServer) {
			// Define the add peer hook
			server.HookAddPeer(func(
				id peer.ID,
				direction network.Direction,
			) {
				peersArray = append(peersArray, id)
			})

			// Define the mock IdentityClient response
			server.GetMockIdentityClient().HookHello(func(
				ctx context.Context,
				in *proto.Status,
				opts ...grpc.CallOption,
			) (*proto.Status, error) {
				// echo back the chain ID
				return in, nil
			})
		},
	)

	connectErr := identityService.handleConnected("TestPeer", network.DirInbound)
	if connectErr == nil {
		t.Fatalf("no connection error occurred")
	}

	assert.ErrorIs(t, connectErr, ErrSelfConnection)

	assert.Len(t, peersArray, 0)
}

// TestHandshake_Errors tests peer connections errors (or echo hello)
func TestHandshake_Errors(t *testing.T) {
	peersArray := make([]peer.ID, 0)
	requesterChainID := int64(1)
	responderChainID := requesterChainID + 1 // different chain ID

	// Create an instance of the identity service
	identityService := newIdentityService(
		// Set the relevant hook responses from the mock server
		func(server *networkTesting.MockNetworkingServer) {
			// Define the add peer hook
			server.HookAddPeer(func(
				id peer.ID,
				direction network.Direction,
			) {
				peersArray = append(peersArray, id)
			})

			// Define the mock IdentityClient response
			server.GetMockIdentityClient().HookHello(func(
				ctx context.Context,
				in *proto.Status,
				opts ...grpc.CallOption,
			) (*proto.Status, error) {
				return &proto.Status{
					Chain: responderChainID,
					Metadata: map[string]string{
						PeerID: "TestPeer1",
					},
					TemporaryDial: false,
				}, nil
			})
		},
	)

	// Set the requester chain ID
	identityService.chainID = requesterChainID

	// Check that there was a chain ID mismatch during handshaking
	connectErr := identityService.handleConnected("TestPeer2", network.DirInbound)
	if connectErr == nil {
		t.Fatalf("no connection error occurred")
	}

	assert.ErrorIs(t, connectErr, ErrInvalidChainID)

	// Make sure no peers have been  added to the base networking server
	assert.Len(t, peersArray, 0)
}
