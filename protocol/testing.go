package protocol

import (
	"math/big"

	"github.com/dogechain-lab/dogechain/blockchain"
	"github.com/dogechain-lab/dogechain/types"
)

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

func (b *mockBlockchain) WriteBlock(block *types.Block, source string) error {
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
		if writeErr := b.WriteBlock(block, WriteBlockSource); writeErr != nil {
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
