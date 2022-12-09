package protocol

import (
	"context"
	"math/big"

	"github.com/dogechain-lab/dogechain/blockchain"
	"github.com/dogechain-lab/dogechain/helper/progress"
	"github.com/dogechain-lab/dogechain/network"
	"github.com/dogechain-lab/dogechain/network/event"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/libp2p/go-libp2p-core/peer"
	rawGrpc "google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

// Syncer is a sync protocol for block downloading
type Syncer interface {
	// Start starts syncer processes
	Start() error
	// Close terminates syncer process
	Close() error
	// GetSyncProgression returns sync progression
	GetSyncProgression() *progress.Progression
	// HasSyncPeer returns whether syncer has the peer syncer can sync with
	HasSyncPeer() bool
	// Sync starts routine to sync blocks
	Sync(func(*types.Block) bool) error
}

// Blockchain is the interface required by the syncer to connect to the blockchain
type Blockchain interface {
	// SubscribeEvents subscribes new blockchain event
	SubscribeEvents() blockchain.Subscription
	Header() *types.Header

	// deprecated methods. Those are old version protocols, keep it only for backward compatible
	CurrentTD() *big.Int
	GetTD(hash types.Hash) (*big.Int, bool)
	GetReceiptsByHash(types.Hash) ([]*types.Receipt, error)
	GetBodyByHash(types.Hash) (*types.Body, bool)
	GetHeaderByHash(types.Hash) (*types.Header, bool)
	GetHeaderByNumber(n uint64) (*types.Header, bool)
	CalculateGasLimit(number uint64) (uint64, error)

	// advance chain methods
	WriteBlock(block *types.Block, source string) error
	VerifyFinalizedBlock(block *types.Block) error

	// GetBlockByNumber returns block by number
	GetBlockByNumber(uint64, bool) (*types.Block, bool)
}

type Network interface {
	// AddrInfo returns Network Info
	AddrInfo() *peer.AddrInfo
	// Peers returns current connected peers
	Peers() []*network.PeerConnInfo
	// IsConnected returns the node is connecting to the peer associated with the given ID
	IsConnected(peerID peer.ID) bool
	// SubscribeCh returns a channel of peer event
	SubscribeCh(context.Context) (<-chan *event.PeerEvent, error)
	// NewTopic Creates New Topic for gossip
	NewTopic(protoID string, obj proto.Message) (*network.Topic, error)
	// RegisterProtocol registers gRPC service
	RegisterProtocol(string, network.Protocol)
	// GetProtoStream returns an active protocol stream if present, otherwise
	// it returns nil
	GetProtoStream(protocol string, peerID peer.ID) *rawGrpc.ClientConn
	// NewProtoConnection opens up a new stream on the set protocol to the peer,
	// and returns a reference to the connection
	NewProtoConnection(protocol string, peerID peer.ID) (*rawGrpc.ClientConn, error)
	// SaveProtocolStream saves stream
	SaveProtocolStream(protocol string, stream *rawGrpc.ClientConn, peerID peer.ID)
	// CloseProtocolStream closes stream
	CloseProtocolStream(protocol string, peerID peer.ID) error
	// ForgetPeer disconnects, remove and forget peer to prevent broadcast discovery to other peers
	ForgetPeer(peer peer.ID, reason string)
}

type Progression interface {
	// StartProgression starts progression
	StartProgression(syncingPeer string, startingBlock uint64, subscription blockchain.Subscription)
	// UpdateHighestProgression updates highest block number
	UpdateHighestProgression(highestBlock uint64)
	// GetProgression returns Progression
	GetProgression() *progress.Progression
	// StopProgression finishes progression
	StopProgression()
}

type SyncPeerService interface {
	// Start starts server
	Start()
	// Close terminates running processes for SyncPeerService
	Close() error

	// deprecated methods

	// SetSyncer sets referent syncer
	SetSyncer(syncer *noForkSyncer)
}

type SyncPeerClient interface {
	// Start processes for SyncPeerClient
	Start() error
	// Close terminates running processes for SyncPeerClient
	Close()
	// GetPeerStatus fetches peer status
	GetPeerStatus(id peer.ID) (*NoForkPeer, error)
	// GetConnectedPeerStatuses fetches the statuses of all connecting peers
	GetConnectedPeerStatuses() []*NoForkPeer
	// GetBlocks returns a stream of blocks from given height to peer's latest
	GetBlocks(ctx context.Context, peerID peer.ID, from uint64, to uint64) ([]*types.Block, error)
	// GetPeerStatusUpdateCh returns a channel of peer's status update
	GetPeerStatusUpdateCh() <-chan *NoForkPeer
	// GetPeerConnectionUpdateEventCh returns peer's connection change event
	GetPeerConnectionUpdateEventCh() <-chan *event.PeerEvent
	// CloseStream close a stream
	CloseStream(peerID peer.ID) error
	// DisablePublishingPeerStatus disables publishing status in syncer topic
	DisablePublishingPeerStatus()
	// EnablePublishingPeerStatus enables publishing status in syncer topic
	EnablePublishingPeerStatus()

	// deprecated methods

	// broadcast block to all its connected peers
	Broadcast(block *types.Block) error
}
