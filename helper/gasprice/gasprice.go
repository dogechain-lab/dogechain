package gasprice

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"sync"

	"github.com/dogechain-lab/dogechain/blockchain"
	"github.com/dogechain-lab/dogechain/chain"
	"github.com/dogechain-lab/dogechain/crypto"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/hashicorp/go-hclog"
	lru "github.com/hashicorp/golang-lru"
)

var (
	Defaults = Config{
		Blocks:           30,
		Percentile:       60,
		MaxHeaderHistory: 1024,
		MaxBlockHistory:  1024,
		MaxPrice:         defaultMaxPrice,
		IgnorePrice:      defaultIgnorePrice,
	}
)

const (
	GWei = 1e9

	sampleNumber = 3 // Number of transactions sampled in a block
)

var (
	defaultMaxPrice    = big.NewInt(750 * GWei)
	defaultIgnorePrice = big.NewInt(50 * GWei)
)

type Config struct {
	Blocks           int
	Percentile       int
	MaxHeaderHistory int
	MaxBlockHistory  int
	Default          *big.Int `toml:",omitempty"`
	MaxPrice         *big.Int `toml:",omitempty"`
	IgnorePrice      *big.Int `toml:",omitempty"`
}

// OracleBackend includes most necessary background APIs for oracle.
type OracleBackend interface {
	Header() *types.Header
	GetBlockByNumber(n uint64, full bool) (*types.Block, bool)
	GetReceiptsByHash(hash types.Hash) ([]*types.Receipt, error)
	// PendingBlockAndReceipts() (*types.Block, types.Receipts)
	ChainID() uint64
	ForksInTime(number uint64) chain.ForksInTime
	SubscribeEvents() blockchain.Subscription
}

// TxpoolBackend includes txpool api for calculating suggestions
type TxpoolBackend interface {
	Pending() map[types.Address][]*types.Transaction
}

// Oracle recommends gas prices based on the content of recent
// blocks. Suitable for both light and full clients.
type Oracle struct {
	backend       OracleBackend
	txpoolBackend TxpoolBackend
	lastHead      types.Hash
	lastPrice     *big.Int
	maxPrice      *big.Int
	ignorePrice   *big.Int
	cacheLock     sync.RWMutex
	fetchLock     sync.Mutex

	checkBlocks, percentile           int
	maxHeaderHistory, maxBlockHistory int

	historyCache *lru.Cache
}

// NewOracle returns a new gasprice oracle which can recommend suitable
// gasprice for newly created transaction.
func NewOracle(
	backend OracleBackend,
	txpoolBackend TxpoolBackend,
	params Config,
	logger hclog.Logger,
) (*Oracle, error) {
	blocks := params.Blocks
	if blocks < 1 {
		return nil, errors.New("invalid gasprice oracle sample blocks")
	}

	percent := params.Percentile
	if percent < 0 || percent > 100 {
		return nil, errors.New("invalid gasprice oracle sample percentile")
	}

	maxPrice := params.MaxPrice
	if maxPrice == nil || maxPrice.Int64() <= 0 {
		return nil, errors.New("invalid gasprice oracle price cap")
	}

	ignorePrice := params.IgnorePrice
	if ignorePrice == nil || ignorePrice.Int64() <= 0 {
		return nil, errors.New("invalid gasprice oracle ignore price")
	}

	maxHeaderHistory := params.MaxHeaderHistory
	if maxHeaderHistory < 1 {
		return nil, errors.New("invalid gasprice oracle max header history")
	}

	maxBlockHistory := params.MaxBlockHistory
	if maxBlockHistory < 1 {
		return nil, errors.New("invalid gasprice oracle max block history")
	}

	cache, err := lru.New(2048)
	if err != nil {
		return nil, fmt.Errorf("gasprice oracle create cache failed: %w", err)
	}

	sub := backend.SubscribeEvents()

	// headEvent := make(chan core.ChainHeadEvent, 1)
	// backend.SubscribeChainHeadEvent(headEvent)

	go func() {
		defer sub.Unsubscribe()

		var lastHead types.Hash

		for ev := range sub.GetEvent() {
			if len(ev.NewChain) == 0 {
				continue
			}

			// it should be only 1 block
			if ev.NewChain[0].ParentHash != lastHead {
				cache.Purge()
			}

			lastHead = ev.NewChain[0].Hash
		}
	}()

	return &Oracle{
		backend:          backend,
		txpoolBackend:    txpoolBackend,
		lastPrice:        params.Default,
		maxPrice:         maxPrice,
		ignorePrice:      ignorePrice,
		checkBlocks:      blocks,
		percentile:       percent,
		maxHeaderHistory: maxHeaderHistory,
		maxBlockHistory:  maxBlockHistory,
		historyCache:     cache,
	}, nil
}

func (oracle *Oracle) getHeadAndPrice() (types.Hash, *big.Int) {
	oracle.cacheLock.RLock()
	defer oracle.cacheLock.RUnlock()

	return oracle.lastHead, oracle.lastPrice
}

func (oracle *Oracle) cacheHeadAndPrice(head types.Hash, price *big.Int) {
	oracle.cacheLock.Lock()
	defer oracle.cacheLock.Unlock()

	oracle.lastHead, oracle.lastPrice = head, price
}

// SuggestTipCap returns a tip cap so that newly created transaction can have a
// very high chance to be included in the following blocks.
func (oracle *Oracle) SuggestTipCap(ctx context.Context) (*big.Int, error) {
	var (
		head     = oracle.backend.Header()
		headHash = head.Hash
	)

	// If the latest gasprice is still available, return it.
	lastHead, lastPrice := oracle.getHeadAndPrice()
	if headHash == lastHead {
		return new(big.Int).Set(lastPrice), nil
	}

	oracle.fetchLock.Lock()
	defer oracle.fetchLock.Unlock()

	// Try checking the cache again, maybe the last fetch fetched what we need
	lastHead, lastPrice = oracle.getHeadAndPrice()
	if headHash == lastHead {
		return new(big.Int).Set(lastPrice), nil
	}

	var (
		sent, exp int
		number    = head.Number
		result    = make(chan results, oracle.checkBlocks)
		quit      = make(chan struct{})
		results   []*big.Int
		chainid   = oracle.backend.ChainID()
	)

	for sent < oracle.checkBlocks && number > 0 {
		go oracle.getBlockValues(
			ctx,
			crypto.NewSigner(oracle.backend.ForksInTime(number), chainid),
			number,
			sampleNumber,
			oracle.ignorePrice,
			result,
			quit,
		)

		sent++
		exp++
		number--
	}

	for exp > 0 {
		res := <-result
		if res.err != nil {
			close(quit)

			return new(big.Int).Set(lastPrice), res.err
		}

		exp--
		// Nothing returned. There are two special cases here:
		// - The block is empty
		// - All the transactions included are sent by the miner itself.
		// In these cases, use the latest calculated price for sampling.
		if len(res.values) == 0 {
			res.values = []*big.Int{lastPrice}
		}
		// Besides, in order to collect enough data for sampling, if nothing
		// meaningful returned, try to query more blocks. But the maximum
		// is 2*checkBlocks.
		if len(res.values) == 1 && len(results)+1+exp < oracle.checkBlocks*2 && number > 0 {
			go oracle.getBlockValues(
				ctx,
				crypto.NewSigner(oracle.backend.ForksInTime(number), chainid),
				number,
				sampleNumber,
				oracle.ignorePrice,
				result,
				quit,
			)

			sent++
			exp++
			number--
		}

		results = append(results, res.values...)
	}

	// first update
	price := lastPrice

	if len(results) > 0 {
		sort.Sort(bigIntArray(results))
		// update with percentile
		price = results[(len(results)-1)*oracle.percentile/100]
	}

	// query current pending txs, if it is overwhelming, give a suggested
	// tx gas price

	// price should not exceed max limit
	if price.Cmp(oracle.maxPrice) > 0 {
		price = new(big.Int).Set(oracle.maxPrice)
	}

	// update cache
	oracle.cacheHeadAndPrice(headHash, price)

	return new(big.Int).Set(price), nil
}

type results struct {
	values []*big.Int
	err    error
}

type txSorter struct {
	txs []*types.Transaction
}

func newSorter(txs []*types.Transaction) *txSorter {
	return &txSorter{
		txs: txs,
	}
}

func (s *txSorter) Len() int { return len(s.txs) }
func (s *txSorter) Swap(i, j int) {
	s.txs[i], s.txs[j] = s.txs[j], s.txs[i]
}
func (s *txSorter) Less(i, j int) bool {
	return s.txs[i].GasPrice.Cmp(s.txs[j].GasPrice) < 0
}

// getBlockPrices calculates the lowest transaction gas price in a given block
// and sends it to the result channel. If the block is empty or all transactions
// are sent by the miner itself(it doesn't make any sense to include this kind of
// transaction prices for sampling), nil gasprice is returned.
func (oracle *Oracle) getBlockValues(
	ctx context.Context,
	signer crypto.TxSigner,
	blockNum uint64,
	limit int,
	ignoreUnder *big.Int,
	result chan results,
	quit chan struct{},
) {
	block, _ := oracle.backend.GetBlockByNumber(blockNum, true)
	if block == nil {
		select {
		case result <- results{nil, errors.New("block not exists")}:
		case <-quit:
		}

		return
	}

	// Sort the transaction by gas price in ascending sort.
	txs := make([]*types.Transaction, len(block.Transactions))
	copy(txs, block.Transactions)

	sorter := newSorter(txs)
	sort.Sort(sorter)

	prices := make([]*big.Int, 0, limit)

	for _, tx := range sorter.txs {
		tip := tx.GasPrice
		if ignoreUnder != nil && tip.Cmp(ignoreUnder) == -1 {
			continue
		}

		sender, err := signer.Sender(tx)
		// ignore validator sending txs, mostly are system contract txns
		if err != nil || sender == block.Header.Miner {
			continue
		}

		prices = append(prices, tip)
		if len(prices) >= limit {
			break
		}
	}

	select {
	case result <- results{prices, nil}:
	case <-quit:
	}
}

type bigIntArray []*big.Int

func (s bigIntArray) Len() int           { return len(s) }
func (s bigIntArray) Less(i, j int) bool { return s[i].Cmp(s[j]) < 0 }
func (s bigIntArray) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
