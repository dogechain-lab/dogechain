package testing

import (
	"context"
	"time"

	"github.com/dogechain-lab/dogechain/helper/telemetry"
	"github.com/dogechain-lab/dogechain/network/client"
	"github.com/dogechain-lab/dogechain/network/event"
	"github.com/dogechain-lab/dogechain/network/proto"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

type MockNetworkingServer struct {
	// Mock identity client that simulates another peer
	mockIdentityClient *MockIdentityClient

	// Mock discovery client that simulates another peer
	mockDiscoveryClient *MockDiscoveryClient

	// Mock libp2p peer metrics
	mockPeerMetrics *MockPeerMetrics

	// Hooks that the test can set //
	// Identity Hooks
	newIdentityClientFn      newIdentityClientDelegate
	disconnectFromPeerFn     disconnectFromPeerDelegate
	addPeerFn                addPeerDelegate
	connectFn                connectDelegate
	updatePendingConnCountFn updatePendingConnCountDelegate
	emitEventFn              emitEventDelegate
	hasFreeConnectionSlotFn  hasFreeConnectionSlotDelegate

	// Discovery Hooks
	newDiscoveryClientFn  newDiscoveryClientDelegate
	getRandomBootnodeFn   getRandomBootnodeDelegate
	addToPeerStoreFn      addToPeerStoreDelegate
	removeFromPeerStoreFn removeFromPeerStoreDelegate
	getPeerInfoFn         getPeerInfoDelegate
	getRandomPeerFn       getRandomPeerDelegate
	peerCountFn           peerCountDelegate
	isBootnodeFn          isBootnodeDelegate
	isStaticPeerFn        isStaticPeerDelegate
	hasPeerFn             hasPeerDelegate

	// tracer
	getTraceFn getTraceDelegate
}

func NewMockNetworkingServer() *MockNetworkingServer {
	return &MockNetworkingServer{
		mockIdentityClient:  &MockIdentityClient{},
		mockDiscoveryClient: &MockDiscoveryClient{},
		mockPeerMetrics:     &MockPeerMetrics{},
	}
}

func (m *MockNetworkingServer) GetMockIdentityClient() *MockIdentityClient {
	return m.mockIdentityClient
}

func (m *MockNetworkingServer) GetMockDiscoveryClient() *MockDiscoveryClient {
	return m.mockDiscoveryClient
}

func (m *MockNetworkingServer) GetMockPeerMetrics() *MockPeerMetrics {
	return m.mockPeerMetrics
}

// Define the mock hooks //
// Required for Identity
type newIdentityClientDelegate func(peer.ID) (client.IdentityClient, error)
type connectDelegate func(addrInfo peer.AddrInfo) error
type disconnectFromPeerDelegate func(peer.ID, string)
type addPeerDelegate func(peer.ID, network.Direction)
type updatePendingConnCountDelegate func(int64, network.Direction)
type emitEventDelegate func(context.Context, *event.PeerEvent)
type hasFreeConnectionSlotDelegate func(network.Direction) bool

// Required for Discovery
type getRandomBootnodeDelegate func() *peer.AddrInfo
type newDiscoveryClientDelegate func(peer.ID) (client.DiscoveryClient, error)
type addToPeerStoreDelegate func(*peer.AddrInfo)
type removeFromPeerStoreDelegate func(peerInfo peer.ID)
type getPeerInfoDelegate func(peer.ID) *peer.AddrInfo
type getRandomPeerDelegate func() *peer.ID
type peerCountDelegate func() int64
type isBootnodeDelegate func(peer.ID) bool
type isStaticPeerDelegate func(peer.ID) bool
type hasPeerDelegate func(peer.ID) bool

// tracer
type getTraceDelegate func() telemetry.Tracer

func (m *MockNetworkingServer) NewIdentityClient(peerID peer.ID) (client.IdentityClient, error) {
	if m.newIdentityClientFn != nil {
		return m.newIdentityClientFn(peerID)
	}

	return m.mockIdentityClient, nil
}

func (m *MockNetworkingServer) HookNewIdentityClient(fn newIdentityClientDelegate) {
	m.newIdentityClientFn = fn
}

func (m *MockNetworkingServer) DisconnectFromPeer(peerID peer.ID, reason string) {
	if m.disconnectFromPeerFn != nil {
		m.disconnectFromPeerFn(peerID, reason)
	}
}

func (m *MockNetworkingServer) HookDisconnectFromPeer(fn disconnectFromPeerDelegate) {
	m.disconnectFromPeerFn = fn
}

func (m *MockNetworkingServer) AddPeer(id peer.ID, direction network.Direction) {
	if m.addPeerFn != nil {
		m.addPeerFn(id, direction)
	}
}

func (m *MockNetworkingServer) HookAddPeer(fn addPeerDelegate) {
	m.addPeerFn = fn
}

func (m *MockNetworkingServer) Connect(addrInfo peer.AddrInfo) error {
	if m.connectFn != nil {
		return m.connectFn(addrInfo)
	}

	return nil
}

func (m *MockNetworkingServer) HookConnect(fn connectDelegate) {
	m.connectFn = fn
}

func (m *MockNetworkingServer) UpdatePendingConnCount(delta int64, direction network.Direction) {
	if m.updatePendingConnCountFn != nil {
		m.updatePendingConnCountFn(delta, direction)
	}
}

func (m *MockNetworkingServer) HookUpdatePendingConnCount(fn updatePendingConnCountDelegate) {
	m.updatePendingConnCountFn = fn
}

func (m *MockNetworkingServer) EmitEvent(ctx context.Context, event *event.PeerEvent) {
	if m.emitEventFn != nil {
		m.emitEventFn(ctx, event)
	}
}

func (m *MockNetworkingServer) HookEmitEvent(fn emitEventDelegate) {
	m.emitEventFn = fn
}

func (m *MockNetworkingServer) HasFreeConnectionSlot(direction network.Direction) bool {
	if m.hasFreeConnectionSlotFn != nil {
		return m.hasFreeConnectionSlotFn(direction)
	}

	return true
}

func (m *MockNetworkingServer) HookHasFreeConnectionSlot(fn hasFreeConnectionSlotDelegate) {
	m.hasFreeConnectionSlotFn = fn
}

func (m *MockNetworkingServer) GetRandomBootnode() *peer.AddrInfo {
	if m.getRandomBootnodeFn != nil {
		return m.getRandomBootnodeFn()
	}

	return nil
}

func (m *MockNetworkingServer) HookGetRandomBootnode(fn getRandomBootnodeDelegate) {
	m.getRandomBootnodeFn = fn
}

func (m *MockNetworkingServer) NewDiscoveryClient(peerID peer.ID) (client.DiscoveryClient, error) {
	if m.newDiscoveryClientFn != nil {
		return m.newDiscoveryClientFn(peerID)
	}

	return m.mockDiscoveryClient, nil
}

func (m *MockNetworkingServer) HookNewDiscoveryClient(fn newDiscoveryClientDelegate) {
	m.newDiscoveryClientFn = fn
}

func (m *MockNetworkingServer) AddToPeerStore(peerInfo *peer.AddrInfo) {
	if m.addToPeerStoreFn != nil {
		m.addToPeerStoreFn(peerInfo)
	}
}

func (m *MockNetworkingServer) HookAddToPeerStore(fn addToPeerStoreDelegate) {
	m.addToPeerStoreFn = fn
}

func (m *MockNetworkingServer) RemoveFromPeerStore(peerID peer.ID) {
	if m.removeFromPeerStoreFn != nil {
		m.removeFromPeerStoreFn(peerID)
	}
}

func (m *MockNetworkingServer) HookRemoveFromPeerStore(fn removeFromPeerStoreDelegate) {
	m.removeFromPeerStoreFn = fn
}

func (m *MockNetworkingServer) GetPeerInfo(peerID peer.ID) *peer.AddrInfo {
	if m.getPeerInfoFn != nil {
		return m.getPeerInfoFn(peerID)
	}

	return nil
}

func (m *MockNetworkingServer) HookGetPeerInfo(fn getPeerInfoDelegate) {
	m.getPeerInfoFn = fn
}

func (m *MockNetworkingServer) GetRandomPeer() *peer.ID {
	if m.getRandomPeerFn != nil {
		return m.getRandomPeerFn()
	}

	return nil
}

func (m *MockNetworkingServer) HookGetRandomPeer(fn getRandomPeerDelegate) {
	m.getRandomPeerFn = fn
}

func (m *MockNetworkingServer) PeerCount() int64 {
	if m.peerCountFn != nil {
		return m.peerCountFn()
	}

	return 0
}

func (m *MockNetworkingServer) HookPeerCount(fn peerCountDelegate) {
	m.peerCountFn = fn
}

func (m *MockNetworkingServer) IsStaticPeer(peerID peer.ID) bool {
	if m.isStaticPeerFn != nil {
		return m.isStaticPeerFn(peerID)
	}

	return false
}

func (m *MockNetworkingServer) HookIsBootnode(fn isBootnodeDelegate) {
	m.isBootnodeFn = fn
}

func (m *MockNetworkingServer) IsBootnode(peerID peer.ID) bool {
	if m.isBootnodeFn != nil {
		return m.isBootnodeFn(peerID)
	}

	return false
}

func (m *MockNetworkingServer) HookIsStaticPeer(fn isStaticPeerDelegate) {
	m.isStaticPeerFn = fn
}

func (m *MockNetworkingServer) HasPeer(peerID peer.ID) bool {
	if m.hasPeerFn != nil {
		return m.hasPeerFn(peerID)
	}

	return false
}

func (m *MockNetworkingServer) HookHasPeer(fn hasPeerDelegate) {
	m.hasPeerFn = fn
}

func (m *MockNetworkingServer) HookGetTracer(fn getTraceDelegate) {
	m.getTraceFn = fn
}

func (m *MockNetworkingServer) GetTracer() telemetry.Tracer {
	if m.getTraceFn != nil {
		return m.getTraceFn()
	}

	return nil
}

// MockIdentityClient mocks an identity client (other peer in the communication)
type MockIdentityClient struct {
	// Hooks that the test can set
	helloFn   helloDelegate
	closeFn   closeDelegate
	isCloseFn isCloseDelegate
}

type helloDelegate func(
	ctx context.Context,
	in *proto.Status,
) (*proto.Status, error)

type closeDelegate func() error
type isCloseDelegate func() bool

func (mic *MockIdentityClient) HookHello(fn helloDelegate) {
	mic.helloFn = fn
}

func (mic *MockIdentityClient) Hello(
	ctx context.Context,
	in *proto.Status,
) (*proto.Status, error) {
	if mic.helloFn != nil {
		return mic.helloFn(ctx, in)
	}

	return nil, nil
}

func (mic *MockIdentityClient) HookClose(fn closeDelegate) {
	mic.closeFn = fn
}

func (mic *MockIdentityClient) Close() error {
	if mic.closeFn != nil {
		return mic.closeFn()
	}

	return nil
}

func (mic *MockIdentityClient) HookIsClose(fn isCloseDelegate) {
	mic.isCloseFn = fn
}

func (mic *MockIdentityClient) IsClose() bool {
	if mic.isCloseFn != nil {
		return mic.isCloseFn()
	}

	return false
}

// MockDiscoveryClient mocks a discovery client (other peer in the communication)
type MockDiscoveryClient struct {
	// Hooks that the test can set
	findPeersFn findPeersDelegate
	// Hooks close function
	closeFn closeDelegate
	// Hooks isClose function
	isCloseFn isCloseDelegate
}

type findPeersDelegate func(
	ctx context.Context,
	in *proto.FindPeersReq,
) (*proto.FindPeersResp, error)

func (mdc *MockDiscoveryClient) HookFindPeers(fn findPeersDelegate) {
	mdc.findPeersFn = fn
}

func (mdc *MockDiscoveryClient) FindPeers(
	ctx context.Context,
	in *proto.FindPeersReq,
) (*proto.FindPeersResp, error) {
	if mdc.findPeersFn != nil {
		return mdc.findPeersFn(ctx, in)
	}

	return nil, nil
}

func (mdc *MockDiscoveryClient) HookClose(fn closeDelegate) {
	mdc.closeFn = fn
}

func (mdc *MockDiscoveryClient) Close() error {
	if mdc.closeFn != nil {
		return mdc.closeFn()
	}

	return nil
}

func (mdc *MockDiscoveryClient) HookIsClose(fn isCloseDelegate) {
	mdc.isCloseFn = fn
}

func (mdc *MockDiscoveryClient) IsClose() bool {
	if mdc.isCloseFn != nil {
		return mdc.isCloseFn()
	}

	return false
}

// MockPeerMetrics is a mock used by the Kademlia routing table
type MockPeerMetrics struct {
	recordLatencyFn     recordLatencyDelegate
	latencyEWMAFn       latencyEWMADelegate
	removeMetricsPeerFn removeMetricsPeerDelegate
}

type recordLatencyDelegate func(id peer.ID, duration time.Duration)
type latencyEWMADelegate func(id peer.ID) time.Duration
type removeMetricsPeerDelegate func(id peer.ID)

func (m *MockPeerMetrics) RecordLatency(id peer.ID, duration time.Duration) {
	if m.recordLatencyFn != nil {
		m.recordLatencyFn(id, duration)
	}
}

func (m *MockPeerMetrics) HookRecordLatency(fn recordLatencyDelegate) {
	m.recordLatencyFn = fn
}

func (m *MockPeerMetrics) LatencyEWMA(id peer.ID) time.Duration {
	if m.latencyEWMAFn != nil {
		return m.latencyEWMAFn(id)
	}

	return 0
}

func (m *MockPeerMetrics) HookLatencyEWMA(fn latencyEWMADelegate) {
	m.latencyEWMAFn = fn
}

func (m *MockPeerMetrics) RemovePeer(id peer.ID) {
	if m.removeMetricsPeerFn != nil {
		m.removeMetricsPeerFn(id)
	}
}

func (m *MockPeerMetrics) HookRemoveMetricsPeer(fn removeMetricsPeerDelegate) {
	m.removeMetricsPeerFn = fn
}
