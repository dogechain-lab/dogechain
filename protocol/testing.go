package protocol

import (
	"context"
	"crypto/rand"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/dogechain-lab/dogechain/blockchain"
	"github.com/dogechain-lab/dogechain/helper/tests"
	"github.com/dogechain-lab/dogechain/network"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

const (
	maxSeed = math.MaxInt32
)

var (
	defaultNetworkConfig = func(c *network.Config) {
		c.NoDiscover = true
	}
)

// createSyncer initialize syncer with server
func createSyncer(t *testing.T, blockchain Blockchain, serverCfg *func(c *network.Config)) *noForkSyncer {
	t.Helper()

	if serverCfg == nil {
		serverCfg = &defaultNetworkConfig
	}

	srv, createErr := network.CreateServer(&network.CreateServerParams{
		ConfigCallback: func(c *network.Config) {
			c.DataDir = t.TempDir()
			(*serverCfg)(c)
		},
	})
	if createErr != nil {
		t.Fatalf("Unable to create networking server, %v", createErr)
	}

	syncer := NewSyncer(hclog.NewNullLogger(), srv, blockchain, 6*time.Second)
	syncer.Start()

	s, ok := syncer.(*noForkSyncer)
	if !ok {
		t.Fatal("syncer not a noForkSyncer")
	}

	return s
}

// WaitUntilProcessedAllEvents waits until syncer finish to process all blockchain events
func WaitUntilProcessedAllEvents(t *testing.T, syncer *noForkSyncer, timeout time.Duration) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	t.Cleanup(func() {
		cancel()
	})

	_, err := tests.RetryUntilTimeout(ctx, func() (interface{}, bool) {
		return nil, len(syncer.blockchain.SubscribeEvents().GetEventCh()) > 0
	})
	assert.NoError(t, err)
}

// WaitUntilProgressionUpdated waits until the syncer's progression current block reaches a target
func WaitUntilProgressionUpdated(t *testing.T, syncer *noForkSyncer, timeout time.Duration, target uint64) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	t.Cleanup(func() {
		cancel()
	})

	_, err := tests.RetryUntilTimeout(ctx, func() (interface{}, bool) {
		return nil, syncer.syncProgression.GetProgression().CurrentBlock < target
	})
	assert.NoError(t, err)
}

// NewRandomChain returns new blockchain with random seed
func NewRandomChain(t *testing.T, height int) Blockchain {
	t.Helper()

	randNum, _ := rand.Int(rand.Reader, big.NewInt(int64(maxSeed)))

	return blockchain.NewTestBlockchain(
		t,
		blockchain.NewTestHeadersWithSeed(
			nil,
			height,
			randNum.Uint64(),
		),
	)
}

// SetupSyncerNetwork connects syncers
func SetupSyncerNetwork(
	t *testing.T,
	chain Blockchain,
	peerChains []Blockchain,
) (syncer *noForkSyncer, peerSyncers []*noForkSyncer) {
	t.Helper()

	syncer = createSyncer(t, chain, nil)
	peerSyncers = make([]*noForkSyncer, len(peerChains))

	for idx, peerChain := range peerChains {
		peerSyncers[idx] = createSyncer(t, peerChain, nil)

		if joinErr := network.JoinAndWait(
			syncer.server,
			peerSyncers[idx].server,
			network.DefaultBufferTimeout,
			network.DefaultJoinTimeout,
		); joinErr != nil {
			t.Fatalf("Unable to join servers, %v", joinErr)
		}
	}

	return syncer, peerSyncers
}

// GenerateNewBlocks returns new blocks from latest block of given chain
func GenerateNewBlocks(t *testing.T, chain Blockchain, num int) []*types.Block {
	t.Helper()

	currentHeight := chain.Header().Number
	oldHeaders := make([]*types.Header, currentHeight+1)

	for i := uint64(1); i <= currentHeight; i++ {
		var ok bool
		oldHeaders[i], ok = chain.GetHeaderByNumber(i)
		assert.Truef(t, ok, "chain should have header at %d, but empty", i)
	}

	headers := blockchain.AppendNewTestHeaders(oldHeaders, num)

	return blockchain.HeadersToBlocks(headers[currentHeight+1:])
}

// GetCurrentStatus return status by latest block in blockchain
func GetCurrentStatus(b Blockchain) *Status {
	return &Status{
		Hash:       b.Header().Hash,
		Number:     b.Header().Number,
		Difficulty: b.CurrentTD(),
	}
}

// HeaderToStatus converts given header to Status
func HeaderToStatus(h *types.Header) *Status {
	var td uint64 = 0
	for i := uint64(1); i <= h.Difficulty; i++ {
		td = td + i
	}

	return &Status{
		Hash:       h.Hash,
		Number:     h.Number,
		Difficulty: big.NewInt(0).SetUint64(td),
	}
}

// mockBlockchain is a mock of blockhain for syncer tests
type mockBlockchain struct {
	blocks []*types.Block

	// fields for new version protocol tests

	subscription                blockchain.Subscription
	headerHandler               func() *types.Header
	getBlockByNumberHandler     func(uint64, bool) (*types.Block, bool)
	verifyFinalizedBlockHandler func(*types.Block) error
	writeBlockHandler           func(*types.Block) error
}

func (b *mockBlockchain) CalculateGasLimit(number uint64) (uint64, error) {
	panic("implement me")
}

func NewMockBlockchain(headers []*types.Header) *mockBlockchain {
	return &mockBlockchain{
		blocks: blockchain.HeadersToBlocks(headers),
	}
}

func (b *mockBlockchain) SubscribeEvents() blockchain.Subscription {
	return b.subscription
}

func (b *mockBlockchain) Header() *types.Header {
	if b.headerHandler != nil {
		return b.headerHandler()
	}

	l := len(b.blocks)
	if l == 0 {
		return nil
	}

	return b.blocks[l-1].Header
}

func (b *mockBlockchain) CurrentTD() *big.Int {
	current := b.Header()
	if current == nil {
		return nil
	}

	td, _ := b.GetTD(current.Hash)

	return td
}

func (b *mockBlockchain) GetTD(hash types.Hash) (*big.Int, bool) {
	var td uint64 = 0
	for _, b := range b.blocks {
		td = td + b.Header.Difficulty

		if b.Header.Hash == hash {
			return big.NewInt(0).SetUint64(td), true
		}
	}

	return nil, false
}

func (b *mockBlockchain) GetReceiptsByHash(types.Hash) ([]*types.Receipt, error) {
	panic("not implement")
}

func (b *mockBlockchain) GetBodyByHash(types.Hash) (*types.Body, bool) {
	return &types.Body{}, true
}

func (b *mockBlockchain) GetHeaderByHash(h types.Hash) (*types.Header, bool) {
	for _, b := range b.blocks {
		if b.Header.Hash == h {
			return b.Header, true
		}
	}

	return nil, false
}

func (b *mockBlockchain) GetHeaderByNumber(n uint64) (*types.Header, bool) {
	for _, b := range b.blocks {
		if b.Header.Number == n {
			return b.Header, true
		}
	}

	return nil, false
}

func (b *mockBlockchain) GetBlockByNumber(n uint64, full bool) (*types.Block, bool) {
	if b.getBlockByNumberHandler != nil {
		return b.getBlockByNumberHandler(n, full)
	}

	for _, b := range b.blocks {
		if b.Number() == n {
			return b, true
		}
	}

	return nil, false
}

func newSimpleHeaderHandler(num uint64) func() *types.Header {
	return func() *types.Header {
		return &types.Header{
			Number: num,
		}
	}
}

func (b *mockBlockchain) WriteBlock(block *types.Block) error {
	b.blocks = append(b.blocks, block)

	if b.writeBlockHandler != nil {
		return b.writeBlockHandler(block)
	}

	return nil
}

func (b *mockBlockchain) VerifyFinalizedBlock(block *types.Block) error {
	if b.verifyFinalizedBlockHandler != nil {
		return b.verifyFinalizedBlockHandler(block)
	}

	return nil
}

func (b *mockBlockchain) WriteBlocks(blocks []*types.Block) error {
	for _, block := range blocks {
		if writeErr := b.WriteBlock(block); writeErr != nil {
			return writeErr
		}
	}

	return nil
}

// mockSubscription is a mock of subscription for blockchain events
type mockSubscription struct {
	eventCh chan *blockchain.Event
}

func NewMockSubscription() *mockSubscription {
	return &mockSubscription{
		eventCh: make(chan *blockchain.Event),
	}
}

func (s *mockSubscription) AppendBlock(block *types.Block) {
	status := HeaderToStatus(block.Header)
	s.eventCh <- &blockchain.Event{
		Difficulty: status.Difficulty,
		NewChain:   []*types.Header{block.Header},
	}
}

func (s *mockSubscription) GetEventCh() chan *blockchain.Event {
	return s.eventCh
}

func (s *mockSubscription) GetEvent() *blockchain.Event {
	return <-s.eventCh
}

func (s *mockSubscription) Close() {
	close(s.eventCh)
}

func createMockBlocks(num int) []*types.Block {
	blocks := make([]*types.Block, num)
	for i := 0; i < num; i++ {
		blocks[i] = &types.Block{
			Header: &types.Header{
				Number: uint64(i + 1),
			},
		}
	}

	return blocks
}
