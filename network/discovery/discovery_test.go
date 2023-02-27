package discovery

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/dogechain-lab/dogechain/helper/tests"
	"github.com/dogechain-lab/dogechain/network/client"
	"github.com/dogechain-lab/dogechain/network/common"
	"github.com/dogechain-lab/dogechain/network/proto"

	"github.com/hashicorp/go-hclog"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"

	networkTesting "github.com/dogechain-lab/dogechain/network/testing"
	ranger "github.com/libp2p/go-cidranger"
	kb "github.com/libp2p/go-libp2p-kbucket"
	libp2pCrypto "github.com/libp2p/go-libp2p/core/crypto"
	ma "github.com/multiformats/go-multiaddr"
)

// newDiscoveryService creates a new discovery service instance
// with mock-able backends
func newDiscoveryService(
	networkingServerCallback func(server *networkTesting.MockNetworkingServer),
) (*DiscoveryService, error) {
	// Setup the mock networking server
	baseServer := networkTesting.NewMockNetworkingServer()

	if networkingServerCallback != nil {
		networkingServerCallback(baseServer)
	}

	// Setup the kademlia routing table
	routingTable, routingErr := kb.NewRoutingTable(
		10,
		kb.ConvertPeerID("ExampleID"),
		time.Minute,
		baseServer.GetMockPeerMetrics(),
		10*time.Second,
		nil,
	)
	if routingErr != nil {
		return nil, routingErr
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &DiscoveryService{
		baseServer:   baseServer,
		logger:       hclog.NewNullLogger(),
		routingTable: routingTable,
		peerAddress:  newPeerAddreStore(),
		ctx:          ctx,
		ctxCancel:    cancel,
	}, nil
}

// getRandomPeers returns random peers, generated on the fly
func getRandomPeers(t *testing.T, count int) []*peer.AddrInfo {
	t.Helper()

	peersInfo := make([]*peer.AddrInfo, 0)

	for i := 0; i < count; i++ {
		info, err := peer.AddrInfoFromP2pAddr(
			tests.GenerateTestMultiAddr(t),
		)
		if err != nil {
			t.Fatalf("unable to generate peer info, %v", err)
		}

		peersInfo = append(peersInfo, info)
	}

	return peersInfo
}

// TestDiscoveryService_BootnodePeerDiscovery makes sure the
// discovery service's peer discovery mechanism through the bootnode works as
// expected
func TestDiscoveryService_BootnodePeerDiscovery(t *testing.T) {
	randomBootnode := &peer.AddrInfo{
		ID: "RandomBootnode",
	}
	randomPeers := getRandomPeers(t, 3)
	peerStore := make([]*peer.AddrInfo, 0)

	// Create an instance of the identity service
	discoveryService, setupErr := newDiscoveryService(
		// Set the relevant hook responses from the mock server
		func(server *networkTesting.MockNetworkingServer) {
			server.HookHasPeer(func(peerID peer.ID) bool {
				return true
			})

			server.HookPeerCount(func() int64 {
				return 0
			})

			// Define the random bootnode hook
			server.HookGetRandomBootnode(func() *peer.AddrInfo {
				return randomBootnode
			})

			// Define the discovery client find peers hook
			server.GetMockDiscoveryClient().HookFindPeers(
				func(
					ctx context.Context,
					in *proto.FindPeersReq,
				) (*proto.FindPeersResp, error) {
					// Encode the response to a string array
					peers := make([]string, len(randomPeers))

					for i, peerInfo := range randomPeers {
						// The peer info needs to be formatted as a MultiAddr
						peers[i] = common.AddrInfoToString(peerInfo)
					}

					return &proto.FindPeersResp{
						Nodes: peers,
					}, nil
				},
			)

			// Define the peer store addition
			server.HookAddToPeerStore(func(info *peer.AddrInfo) {
				peerStore = append(peerStore, info)
			})
		},
	)
	if setupErr != nil {
		t.Fatalf("Unable to setup the discovery service")
	}

	t.Cleanup(func() {
		discoveryService.Close()
	})

	// Run the discovery service
	discoveryService.bootnodePeerDiscovery()

	// Make sure the bootnode peers are added to the peer store
	assert.Len(t, peerStore, len(randomPeers))

	for indx, randomPeer := range randomPeers {
		assert.Equal(t, randomPeer.ID, peerStore[indx].ID)
	}
}

// TestDiscoveryService_AddToTable tests that peers are added correctly
// both to the discovery routing table, and the networking server's peer store
func TestDiscoveryService_AddToTable(t *testing.T) {
	randomPeer := getRandomPeers(t, 1)[0]
	peerStore := make(map[peer.ID]*peer.AddrInfo)

	addToPeerStoreHook := func(info *peer.AddrInfo) {
		peerStore[info.ID] = info
	}

	removeFromPeerStoreHook := func(peerID peer.ID) {
		delete(peerStore, peerID)
	}

	highLatencyHook := func(id peer.ID) time.Duration {
		// Set a high latency value to trigger an error
		return 24 * time.Hour
	}

	lowLatencyHook := func(id peer.ID) time.Duration {
		// Set a non-existing latency value
		return 0
	}

	testTable := []struct {
		name          string
		latencyHook   func(id peer.ID) time.Duration
		shouldSucceed bool
	}{
		{
			"routing table and peer store updated",
			lowLatencyHook,
			true,
		},
		{
			"peer store should not be updated due to error",
			highLatencyHook,
			false,
		},
	}

	for _, testCase := range testTable {
		t.Run(testCase.name, func(t *testing.T) {
			// Create an instance of the identity service
			discoveryService, setupErr := newDiscoveryService(
				// Set the relevant hook responses from the mock server
				func(server *networkTesting.MockNetworkingServer) {
					// Define the peer store addition
					server.HookAddToPeerStore(addToPeerStoreHook)

					// Define the peer store deletion
					server.HookRemoveFromPeerStore(removeFromPeerStoreHook)

					// Define the routing table latency hook
					server.GetMockPeerMetrics().HookLatencyEWMA(testCase.latencyHook)
				},
			)
			if setupErr != nil {
				t.Fatalf("Unable to setup the discovery service")
			}

			t.Cleanup(func() {
				discoveryService.Close()
			})

			// Run the main method
			additionErr := discoveryService.addToTable(randomPeer)

			if testCase.shouldSucceed {
				assert.NoError(t, additionErr)
				assert.Len(t, peerStore, 1)
				assert.Equal(t, randomPeer, peerStore[randomPeer.ID])
			} else {
				assert.Error(t, additionErr)
				assert.Len(t, peerStore, 0)
			}
		})
	}
}

// TestDiscoveryService_RegularPeerDiscoveryUnconnected makes sure the peers who disconnected
// in the middle of peer discovery are not queried for their peer sets
func TestDiscoveryService_RegularPeerDiscoveryUnconnected(t *testing.T) {
	randomPeer := getRandomPeers(t, 1)[0]
	peerStore := make(map[peer.ID]*peer.AddrInfo)

	// Create an instance of the identity service
	discoveryService, setupErr := newDiscoveryService(
		// Set the relevant hook responses from the mock server
		func(server *networkTesting.MockNetworkingServer) {
			// Define the random peer hook
			server.HookGetRandomPeer(func() *peer.ID {
				return &randomPeer.ID
			})

			// Define the new discovery client creation
			server.HookNewDiscoveryClient(func(id peer.ID) (client.DiscoveryClient, error) {
				return nil, errors.New("peer is not connected anymore")
			})

			// Define the peer store addition
			server.HookAddToPeerStore(func(info *peer.AddrInfo) {
				peerStore[info.ID] = info
			})
		},
	)
	if setupErr != nil {
		t.Fatalf("Unable to setup the discovery service")
	}

	t.Cleanup(func() {
		discoveryService.Close()
	})

	// Run the regular peer discovery method
	discoveryService.regularPeerDiscovery()

	// Make sure that no peers were added to the peer store
	assert.Len(t, peerStore, 0)
}

func TestDiscoveryService_IgnorePeer(t *testing.T) {
	randomPeers := getRandomPeers(t, 3)
	bootnode := randomPeers[0]

	ignorePeer := func() *peer.AddrInfo {
		priv, _, err := libp2pCrypto.GenerateKeyPair(libp2pCrypto.Secp256k1, 256)
		if err != nil {
			t.Fatalf("Unable to generate key pair, %v", err)
		}

		nodeID, err := peer.IDFromPrivateKey(priv)
		assert.NoError(t, err)

		return &peer.AddrInfo{
			ID:    nodeID,
			Addrs: []ma.Multiaddr{ma.StringCast(fmt.Sprintf("/ip4/192.168.1.1/tcp/1234/p2p/%s", nodeID))},
		}
	}()

	randomPeers = append(randomPeers, ignorePeer)

	peerStore := make(map[peer.ID]*peer.AddrInfo)

	// Create an instance of the identity service
	discoveryService, setupErr := newDiscoveryService(
		// Set the relevant hook responses from the mock server
		func(server *networkTesting.MockNetworkingServer) {
			server.HookHasPeer(func(peerID peer.ID) bool {
				return true
			})

			server.HookPeerCount(func() int64 {
				return 0
			})

			// Define the random bootnode hook
			server.HookGetRandomBootnode(func() *peer.AddrInfo {
				return bootnode
			})

			// Define the discovery client find peers hook
			server.GetMockDiscoveryClient().HookFindPeers(
				func(
					ctx context.Context,
					in *proto.FindPeersReq,
				) (*proto.FindPeersResp, error) {
					// Encode the response to a string array
					peers := make([]string, len(randomPeers))

					for i, peerInfo := range randomPeers {
						// The peer info needs to be formatted as a MultiAddr
						peers[i] = common.AddrInfoToString(peerInfo)
					}

					return &proto.FindPeersResp{
						Nodes: peers,
					}, nil
				},
			)

			// Define the peer store addition
			server.HookAddToPeerStore(func(info *peer.AddrInfo) {
				peerStore[info.ID] = info
			})
		},
	)
	if setupErr != nil {
		t.Fatalf("Unable to setup the discovery service")
	}

	t.Cleanup(func() {
		discoveryService.Close()
	})

	// add ignore cidr
	_, network, _ := net.ParseCIDR("192.168.1.0/24")

	ignoreRange := ranger.NewPCTrieRanger()
	ignoreRange.Insert(ranger.NewBasicRangerEntry(*network))

	discoveryService.ignoreCIDR = ignoreRange

	discoveryService.bootnodePeerDiscovery()

	// check ignore peer not added
	assert.Len(t, peerStore, 3)
	assert.NotContains(t, peerStore, ignorePeer.ID)
}
