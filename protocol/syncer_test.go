package protocol

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/dogechain-lab/dogechain/blockchain"
	"github.com/dogechain-lab/dogechain/network"
	"github.com/dogechain-lab/dogechain/protocol/proto"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	anypb "google.golang.org/protobuf/types/known/anypb"
)

func TestNilPointerAttackFromFaultyPeer(t *testing.T) {
	tests := []struct {
		name              string
		headers           []*types.Header
		peerHeaders       []*types.Header
		numNewBlocks      int
		testBroadcastFunc func(s *noForkSyncer, b *types.Block)
	}{
		{
			name:              "should not crash even notify raw data is nil",
			headers:           blockchain.NewTestHeadersWithSeed(nil, 3, 0),
			peerHeaders:       blockchain.NewTestHeadersWithSeed(nil, 1, 0),
			numNewBlocks:      1,
			testBroadcastFunc: broadcastNilRawData,
		},
		{
			name:              "should not crash even notify status is nil",
			headers:           blockchain.NewTestHeadersWithSeed(nil, 5, 0),
			peerHeaders:       blockchain.NewTestHeadersWithSeed(nil, 1, 0),
			numNewBlocks:      1,
			testBroadcastFunc: broadcastNilStatusData,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain, peerChain := NewMockBlockchain(tt.headers), NewMockBlockchain(tt.peerHeaders)

			_, peerSyncers := SetupSyncerNetwork(t, chain, []Blockchain{peerChain})
			peerSyncer := peerSyncers[0]

			newBlocks := GenerateNewBlocks(t, peerChain, tt.numNewBlocks)

			for _, newBlock := range newBlocks {
				assert.NoError(t, peerSyncer.blockchain.VerifyFinalizedBlock(newBlock))
				assert.NoError(t, peerSyncer.blockchain.WriteBlock(newBlock))
			}

			for _, b := range newBlocks {
				assert.NotPanics(t, func() {
					tt.testBroadcastFunc(peerSyncer, b)
				})
			}
		})
	}
}

func broadcastNilRawData(s *noForkSyncer, b *types.Block) {
	// Get the chain difficulty associated with block
	td, ok := s.blockchain.GetTD(b.Hash())
	if !ok {
		// not supposed to happen
		s.logger.Error("total difficulty not found", "block number", b.Number())

		return
	}

	// broadcast the new block to all the peers
	req := &proto.NotifyReq{
		Status: &proto.V1Status{
			Hash:       b.Hash().String(),
			Number:     b.Number(),
			Difficulty: td.String(),
		},
		Raw: nil,
	}

	s.peerMap.Range(func(peerID, peer interface{}) bool {
		if _, err := peer.(*SyncPeer).client.Notify(context.Background(), req); err != nil {
			s.logger.Error("failed to notify", "err", err)
		}

		return true
	})
}

func broadcastNilStatusData(s *noForkSyncer, b *types.Block) {
	// broadcast the new block to all the peers
	req := &proto.NotifyReq{
		Status: nil,
		Raw: &anypb.Any{
			Value: b.MarshalRLP(),
		},
	}

	s.peerMap.Range(func(peerID, peer interface{}) bool {
		if _, err := peer.(*SyncPeer).client.Notify(context.Background(), req); err != nil {
			s.logger.Error("failed to notify", "err", err)
		}

		return true
	})
}

func TestSyncer_GetSyncProgression(t *testing.T) {
	initialChainSize := 10
	targetChainSize := 1000

	existingChain := blockchain.NewTestHeadersWithSeed(nil, initialChainSize, 0)
	syncerChain := NewMockBlockchain(existingChain)
	syncer := createSyncer(t, syncerChain, nil)

	syncHeaders := blockchain.NewTestHeadersWithSeed(nil, targetChainSize, 0)
	syncBlocks := blockchain.HeadersToBlocks(syncHeaders)

	syncer.syncProgression.StartProgression(uint64(initialChainSize), syncerChain.SubscribeEvents())

	if syncer.GetSyncProgression() == nil {
		t.Fatalf("Unable to start progression")
	}

	assert.Equal(t, uint64(initialChainSize), syncer.syncProgression.GetProgression().StartingBlock)

	syncer.syncProgression.UpdateHighestProgression(uint64(targetChainSize))

	assert.Equal(t, uint64(targetChainSize), syncer.syncProgression.GetProgression().HighestBlock)

	writeErr := syncerChain.WriteBlocks(syncBlocks[initialChainSize+1:])

	assert.NoError(t, writeErr)

	WaitUntilProgressionUpdated(t, syncer, 15*time.Second, uint64(targetChainSize-1))
	assert.Equal(t, uint64(targetChainSize-1), syncer.syncProgression.GetProgression().CurrentBlock)

	syncer.syncProgression.StopProgression()
}

type mockBlockStore struct {
	blocks       []*types.Block
	subscription *blockchain.MockSubscription
	td           *big.Int
}

func (m *mockBlockStore) CalculateGasLimit(number uint64) (uint64, error) {
	panic("implement me")
}

func newMockBlockStore() *mockBlockStore {
	bs := &mockBlockStore{
		blocks:       make([]*types.Block, 0),
		subscription: blockchain.NewMockSubscription(),
		td:           big.NewInt(1),
	}

	return bs
}

func (m *mockBlockStore) Header() *types.Header {
	return m.blocks[len(m.blocks)-1].Header
}
func (m *mockBlockStore) GetHeaderByNumber(n uint64) (*types.Header, bool) {
	b, ok := m.GetBlockByNumber(n, false)
	if !ok {
		return nil, false
	}

	return b.Header, true
}
func (m *mockBlockStore) GetBlockByNumber(blockNumber uint64, full bool) (*types.Block, bool) {
	for _, b := range m.blocks {
		if b.Number() == blockNumber {
			return b, true
		}
	}

	return nil, false
}
func (m *mockBlockStore) SubscribeEvents() blockchain.Subscription {
	return m.subscription
}
func (m *mockBlockStore) GetReceiptsByHash(types.Hash) ([]*types.Receipt, error) {
	return nil, nil
}

func (m *mockBlockStore) GetHeaderByHash(hash types.Hash) (*types.Header, bool) {
	for _, b := range m.blocks {
		header := b.Header.ComputeHash()
		if header.Hash == hash {
			return header, true
		}
	}

	return nil, true
}
func (m *mockBlockStore) GetBodyByHash(hash types.Hash) (*types.Body, bool) {
	for _, b := range m.blocks {
		if b.Hash() == hash {
			return b.Body(), true
		}
	}

	return nil, true
}

func (m *mockBlockStore) WriteBlocks(blocks []*types.Block) error {
	for _, block := range blocks {
		if writeErr := m.WriteBlock(block, WriteBlockSource); writeErr != nil {
			return writeErr
		}
	}

	return nil
}

func (m *mockBlockStore) WriteBlock(block *types.Block, source string) error {
	m.td.Add(m.td, big.NewInt(int64(block.Header.Difficulty)))
	m.blocks = append(m.blocks, block)

	return nil
}

func (m *mockBlockStore) VerifyFinalizedBlock(block *types.Block) error {
	return nil
}

func (m *mockBlockStore) CurrentTD() *big.Int {
	return m.td
}

func (m *mockBlockStore) GetTD(hash types.Hash) (*big.Int, bool) {
	return m.td, false
}

func createGenesisBlock() []*types.Block {
	blocks := make([]*types.Block, 0)
	genesis := &types.Header{Difficulty: 1, Number: 0}
	genesis.ComputeHash()

	b := &types.Block{
		Header: genesis,
	}
	blocks = append(blocks, b)

	return blocks
}

func createBlockStores(count int) (bStore []*mockBlockStore) {
	bStore = make([]*mockBlockStore, count)
	for i := 0; i < count; i++ {
		bStore[i] = newMockBlockStore()
	}

	return
}

// createNetworkServers is a helper function for generating network servers
func createNetworkServers(t *testing.T, count int, conf func(c *network.Config)) []*network.Server {
	t.Helper()

	networkServers := make([]*network.Server, count)

	for indx := 0; indx < count; indx++ {
		server, createErr := network.CreateServer(&network.CreateServerParams{ConfigCallback: conf})
		if createErr != nil {
			t.Fatalf("Unable to create network servers, %v", createErr)
		}

		networkServers[indx] = server
	}

	return networkServers
}

// createSyncers is a helper function for generating syncers. Servers and BlockStores should be at least the length
// of count
func createSyncers(count int, servers []*network.Server, blockStores []*mockBlockStore) []*noForkSyncer {
	syncers := make([]*noForkSyncer, count)

	for indx := 0; indx < count; indx++ {
		s := NewSyncer(
			hclog.NewNullLogger(),
			servers[indx],
			blockStores[indx],
			6*time.Second,
		)
		syncer, _ := s.(*noForkSyncer)
		syncers[indx] = syncer
	}

	return syncers
}
