package txpool

import (
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"go.uber.org/atomic"

	"github.com/dogechain-lab/dogechain/blockchain"
	"github.com/dogechain-lab/dogechain/chain"
	"github.com/dogechain-lab/dogechain/network"
	"github.com/dogechain-lab/dogechain/state"
	"github.com/dogechain-lab/dogechain/txpool/proto"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/hashicorp/go-hclog"
	"github.com/libp2p/go-libp2p-core/peer"
	"google.golang.org/grpc"
)

const (
	txSlotSize  = 32 * 1024  // 32kB
	txMaxSize   = 128 * 1024 //128Kb
	topicNameV1 = "txpool/0.1"
)

const (
	_ddosThreshold      = 3               // >= 3/minute is a ddos attack
	_ddosReduceCount    = 5               // contract ddos count reduction
	_ddosReduceDuration = 1 * time.Minute // trigger for ddos count reduction
)

// errors
var (
	ErrIntrinsicGas        = errors.New("intrinsic gas too low")
	ErrBlockLimitExceeded  = errors.New("exceeds block gas limit")
	ErrNegativeValue       = errors.New("negative value")
	ErrExtractSignature    = errors.New("cannot extract signature")
	ErrInvalidSender       = errors.New("invalid sender")
	ErrTxPoolOverflow      = errors.New("txpool is full")
	ErrUnderpriced         = errors.New("transaction underpriced")
	ErrNonceTooLow         = errors.New("nonce too low")
	ErrInsufficientFunds   = errors.New("insufficient funds for gas * price + value")
	ErrInvalidAccountState = errors.New("invalid account state")
	ErrAlreadyKnown        = errors.New("already known")
	ErrOversizedData       = errors.New("oversized data")
	ErrReplaceUnderpriced  = errors.New("replacement transaction underpriced")
	ErrBlackList           = errors.New("address in blacklist")
	ErrContractDDOSList    = errors.New("contract in ddos list")
	ErrTxPoolClosed        = errors.New("txpool is close")
)

// indicates origin of a transaction
type txOrigin int

const (
	local  txOrigin = iota // json-RPC/gRPC endpoints
	gossip                 // gossip protocol
	reorg                  // legacy code
)

func (o txOrigin) String() (s string) {
	switch o {
	case local:
		s = "local"
	case gossip:
		s = "gossip"
	case reorg:
		s = "reorg"
	}

	return
}

// store interface defines State helper methods the Txpool should have access to
type store interface {
	Header() *types.Header
	GetNonce(root types.Hash, addr types.Address) uint64
	GetBalance(root types.Hash, addr types.Address) (*big.Int, error)
	GetBlockByHash(types.Hash, bool) (*types.Block, bool)
}

type signer interface {
	Sender(tx *types.Transaction) (types.Address, error)
}

type Config struct {
	PriceLimit            uint64
	MaxSlots              uint64
	Sealing               bool
	PruneTickSeconds      uint64
	PromoteOutdateSeconds uint64
	BlackList             []types.Address
	DDOSProtection        bool
}

/* All requests are passed to the main loop
through their designated channels. */

// An enqueueRequest is created for any transaction
// meant to be enqueued onto some account.
// This request is created for (new) transactions
// that passed validation in addTx.
type enqueueRequest struct {
	tx *types.Transaction
}

// A promoteRequest is created each time some account
// is eligible for promotion. This request is signaled
// on 2 occasions:
//
// 1. When an enqueued transaction's nonce is
// not greater than the expected (account's nextNonce).
// == 	nextNonce	- transaction is expected (addTx)
// <	nextNonce	- transaction was demoted (Demote)
//
// 2. When an account's nextNonce is updated (during ResetWithHeader)
// and the first enqueued transaction matches the new nonce.
type promoteRequest struct {
	account types.Address
}

// TxPool is a module that handles pending transactions.
// All transactions are handled within their respective accounts.
// An account contains 2 queues a transaction needs to go through:
//
// 1. Enqueued (entry point)
// 2. Promoted (exit point)
//
// both queues are min nonce ordered
//
// When consensus needs to process promoted transactions,
// the pool generates a queue of "executable" transactions. These
// transactions are the first-in-line of some promoted queue,
// ready to be written to the state (primaries).
//
// TODO: Refactor its interface, only expose input methods and events
// subscription for those who interest in. Its state shouldn't be
// manipulated by other components. This means it is self-contained
// and self-consistent. Get enough promotable txs once and for all.
// Enough is enough, so we could keep it consise and bug-free.
type TxPool struct {
	logger hclog.Logger
	signer signer
	forks  chain.ForksInTime
	store  store

	// map of all accounts registered by the pool
	accounts *accountsMap

	// all the primaries sorted by max gas price
	executables *pricedQueue

	// lookup map keeping track of all
	// transactions present in the pool
	index lookupMap

	// networking stack
	topic network.Topic

	// gauge for measuring pool capacity
	gauge slotGauge

	// priceLimit is a lower threshold for gas price
	priceLimit uint64

	// channels on which the pool's event loop
	// does dispatching/handling requests.
	enqueueReqCh chan enqueueRequest
	promoteReqCh chan promoteRequest

	// shutdown channel
	shutdownCh chan struct{}

	// flag indicating if the current node is a sealer,
	// and should therefore gossip transactions
	sealing bool

	// prometheus API
	metrics *Metrics

	// Event manager for txpool events
	eventManager *eventManager

	// indicates which txpool operator commands should be implemented
	proto.UnimplementedTxnPoolOperatorServer

	// pruning configs
	// ticker for pruning account outdated transactions
	pruneAccountTicker     *time.Ticker
	pruneTick              time.Duration
	promoteOutdateDuration time.Duration

	// some very bad guys whose txs should never be included
	blacklist map[types.Address]struct{}
	// ddos protection fields
	ddosProtection      bool         // enable ddos protection
	ddosReductionTicker *time.Ticker // ddos reduction ticker for releasing from imprisonment
	ddosContracts       sync.Map     // ddos contract caching
	ddosWhiteList       sync.Map     // ddos contract white list escaping

	// close flag
	isClosed *atomic.Bool
}

// NewTxPool returns a new pool for processing incoming transactions.
func NewTxPool(
	logger hclog.Logger,
	forks chain.ForksInTime,
	store store,
	grpcServer *grpc.Server,
	network network.Server,
	metrics *Metrics,
	config *Config,
) (*TxPool, error) {
	var (
		pruneTickSeconds      = config.PruneTickSeconds
		promoteOutdateSeconds = config.PromoteOutdateSeconds
		maxSlot               = config.MaxSlots
	)

	if pruneTickSeconds == 0 {
		pruneTickSeconds = DefaultPruneTickSeconds
	}

	if promoteOutdateSeconds == 0 {
		promoteOutdateSeconds = DefaultPromoteOutdateSeconds
	}

	if maxSlot == 0 {
		maxSlot = DefaultMaxSlots
	}

	pool := &TxPool{
		logger:                 logger.Named("txpool"),
		forks:                  forks,
		store:                  store,
		metrics:                metrics,
		accounts:               newAccountsMap(),
		executables:            newPricedQueue(),
		index:                  lookupMap{all: make(map[types.Hash]*types.Transaction)},
		gauge:                  slotGauge{height: 0, max: maxSlot},
		priceLimit:             config.PriceLimit,
		pruneTick:              time.Second * time.Duration(pruneTickSeconds),
		promoteOutdateDuration: time.Second * time.Duration(promoteOutdateSeconds),
		ddosProtection:         config.DDOSProtection,
		isClosed:               atomic.NewBool(false),
	}

	pool.SetSealing(config.Sealing) // sealing flag

	// Attach the event manager
	pool.eventManager = newEventManager(pool.logger)

	if network != nil {
		// subscribe to the gossip protocol
		topic, err := network.NewTopic(topicNameV1, &proto.Txn{})
		if err != nil {
			return nil, err
		}

		// subscribe txpool topic to make a full-message peerings
		if subscribeErr := topic.Subscribe(pool.addGossipTx); subscribeErr != nil {
			return nil, fmt.Errorf("unable to subscribe to gossip topic, %w", subscribeErr)
		}

		pool.topic = topic
	}

	if grpcServer != nil {
		proto.RegisterTxnPoolOperatorServer(grpcServer, pool)
	}

	// initialise channels
	pool.enqueueReqCh = make(chan enqueueRequest)
	pool.promoteReqCh = make(chan promoteRequest)
	pool.shutdownCh = make(chan struct{})

	// blacklist
	pool.blacklist = make(map[types.Address]struct{})
	for _, addr := range config.BlackList {
		pool.blacklist[addr] = struct{}{}
	}

	return pool, nil
}

// SetSealing sets the sealing flag
func (p *TxPool) SetSealing(sealing bool) {
	p.sealing = sealing
}

// sealing returns the current set sealing flag
func (p *TxPool) getSealing() bool {
	return p.sealing
}

// Start runs the pool's main loop in the background.
// On each request received, the appropriate handler
// is invoked in a separate goroutine.
func (p *TxPool) Start() {
	// set default value of txpool transactions gauge
	p.metrics.SetPendingTxs(0)
	p.metrics.SetEnqueueTxs(0)

	p.pruneAccountTicker = time.NewTicker(p.pruneTick)
	p.ddosReductionTicker = time.NewTicker(_ddosReduceDuration)

	go func() {
		for {
			select {
			case <-p.shutdownCh:
				return
			case req, ok := <-p.enqueueReqCh:
				if ok {
					go p.handleEnqueueRequest(req)
				}
			case req, ok := <-p.promoteReqCh:
				if ok {
					go p.handlePromoteRequest(req)
				}
			case _, ok := <-p.pruneAccountTicker.C:
				if ok { // readable
					go p.pruneStaleAccounts()
				}
			case _, ok := <-p.ddosReductionTicker.C:
				if ok {
					// go p.reduceDDOSCounts()
				}
			}
		}
	}()
}

// Close shuts down the pool's main loop.
func (p *TxPool) Close() {
	if !p.isClosed.CAS(false, true) {
		p.logger.Error("txpool is Closed")

		return
	}

	p.ddosReductionTicker.Stop()

	p.logger.Info("txpool close pruneAccountTicker")
	p.pruneAccountTicker.Stop()
	p.eventManager.Close()

	p.logger.Info("txpool close topic")

	if p.topic != nil {
		p.topic.Close()
	}

	p.logger.Info("txpool close all channels")
	// close all channels
	close(p.enqueueReqCh)
	close(p.promoteReqCh)
	close(p.shutdownCh)
}

// SetSigner sets the signer the pool will use
// to validate a transaction's signature.
func (p *TxPool) SetSigner(s signer) {
	p.signer = s
}

// AddTx adds a new transaction to the pool (sent from json-RPC/gRPC endpoints)
// and broadcasts it to the network (if enabled).
func (p *TxPool) AddTx(tx *types.Transaction) error {
	if p.isClosed.Load() {
		p.logger.Error("txpool is Closed")

		return ErrTxPoolClosed
	}

	if err := p.addTx(local, tx); err != nil {
		p.logger.Error("failed to add tx", "err", err)

		return err
	}

	// broadcast the transaction only if a topic
	// subscription is present
	if p.topic != nil {
		tx := &proto.Txn{
			Raw: &any.Any{
				Value: tx.MarshalRLP(),
			},
		}

		if err := p.topic.Publish(tx); err != nil {
			p.logger.Error("failed to topic tx", "err", err)
		}
	}

	return nil
}

// Prepare generates all the transactions
// ready for execution. (primaries)
func (p *TxPool) Prepare() {
	// clear from previous round
	if p.executables.length() != 0 {
		p.executables.clear()
	}

	// fetch primary from each account
	primaries := p.accounts.getPrimaries()

	// push primaries to the executables queue
	for _, tx := range primaries {
		p.executables.push(tx)
	}
}

// Pop returns the best-price selected
// transaction ready for execution.
func (p *TxPool) Pop() *types.Transaction {
	// Popping the executables queue
	// does not remove the actual tx
	// from the pool.
	// The executables queue just provides
	// insight into which account has the
	// highest priced tx (head of promoted queue)
	return p.executables.pop()
}

// RemoveExecuted removes the executed transaction from promoted queue
//
// Will update executables with the next primary
// from that account (if any).
func (p *TxPool) RemoveExecuted(tx *types.Transaction) {
	// fetch the associated account
	account := p.accounts.get(tx.From)

	account.promoted.lock(true)
	defer account.promoted.unlock()

	// pop the top most promoted tx
	account.promoted.pop()

	p.logger.Debug("excutables pop out the max price transaction", "hash", tx.Hash(), "from", tx.From)

	// update state
	p.gauge.decrease(slotsRequired(tx))

	// update metrics
	p.metrics.AddPendingTxs(-1)

	// update executables
	if tx := account.promoted.peek(); tx != nil {
		p.logger.Debug("excutables push in another transaction", "hash", tx.Hash(), "from", tx.From)
		p.executables.push(tx)
	}
}

// DemoteAllPromoted clears all promoted transactions of the account which
// might be not promotable
//
// clears all promoted transactions of the account, re-add them to the txpool,
// and reset the nonce
func (p *TxPool) DemoteAllPromoted(tx *types.Transaction, correctNonce uint64) {
	// fetch associated account
	account := p.accounts.get(tx.From)

	// should lock to rewrite other transactions
	account.promoted.lock(true)
	defer account.promoted.unlock()

	// reset account nonce to the correct one
	account.setNonce(correctNonce)

	if account.promoted.length() == 0 {
		return
	}

	// clear it
	txs := account.promoted.Clear()
	p.index.remove(txs...)
	// update metrics and gauge
	p.metrics.AddPendingTxs(-1 * float64(len(txs)))
	p.gauge.decrease(slotsRequired(txs...))
	// signal events
	p.eventManager.signalEvent(proto.EventType_DEMOTED, toHash(txs...)...)

	go func(txs []*types.Transaction) {
		// retry enqueue, and broadcast
		for _, tx := range txs {
			//nolint:errcheck
			p.AddTx(tx)
		}
	}(txs)
}

// Drop clears the entire account associated with the given transaction
// and reverts its next (expected) nonce.
func (p *TxPool) Drop(tx *types.Transaction) {
	// fetch associated account
	account := p.accounts.get(tx.From)

	account.promoted.lock(true)
	account.enqueued.lock(true)

	// num of all txs dropped
	droppedCount := 0

	// pool resource cleanup
	clearAccountQueue := func(txs []*types.Transaction) {
		p.index.remove(txs...)
		p.gauge.decrease(slotsRequired(txs...))

		// increase counter
		droppedCount += len(txs)
	}

	defer func() {
		account.enqueued.unlock()
		account.promoted.unlock()
	}()

	// rollback nonce
	nextNonce := tx.Nonce
	account.setNonce(nextNonce)

	// drop promoted
	dropped := account.promoted.Clear()
	clearAccountQueue(dropped)

	// update metrics
	p.metrics.AddPendingTxs(float64(-1 * len(dropped)))

	// drop enqueued
	dropped = account.enqueued.Clear()
	clearAccountQueue(dropped)

	// update metrics
	p.metrics.AddEnqueueTxs(float64(-1 * len(dropped)))

	p.eventManager.signalEvent(proto.EventType_DROPPED, tx.Hash())
	p.logger.Debug("dropped account txs",
		"num", droppedCount,
		"next_nonce", nextNonce,
		"address", tx.From.String(),
	)
}

// ResetWithHeaders processes the transactions from the new
// headers to sync the pool with the new state.
func (p *TxPool) ResetWithHeaders(headers ...*types.Header) {
	e := &blockchain.Event{
		NewChain: headers,
	}

	// process the txs in the event
	// to make sure the pool is up-to-date
	p.processEvent(e)
}

// processEvent collects the latest nonces for each account containted
// in the received event. Resets all known accounts with the new nonce.
func (p *TxPool) processEvent(event *blockchain.Event) {
	oldTxs := make(map[types.Hash]*types.Transaction)

	// Legacy reorg logic //
	for _, header := range event.OldChain {
		// transactions to be returned to the pool
		block, ok := p.store.GetBlockByHash(header.Hash, true)
		if !ok {
			continue
		}

		for _, tx := range block.Transactions {
			oldTxs[tx.Hash()] = tx
		}
	}

	// Grab the latest state root now that the block has been inserted
	stateRoot := p.store.Header().StateRoot
	stateNonces := make(map[types.Address]uint64)

	// discover latest (next) nonces for all accounts
	for _, header := range event.NewChain {
		block, ok := p.store.GetBlockByHash(header.Hash, true)
		if !ok {
			p.logger.Error("could not find block in store", "hash", header.Hash.String())

			continue
		}

		// remove mined txs from the lookup map
		p.index.remove(block.Transactions...)

		// etract latest nonces
		for _, tx := range block.Transactions {
			addr := tx.From

			// skip already processed accounts
			if _, processed := stateNonces[addr]; processed {
				continue
			}

			// fetch latest nonce from the state
			latestNonce := p.store.GetNonce(stateRoot, addr)

			// update the result map
			stateNonces[addr] = latestNonce

			// Legacy reorg logic //
			// Update the addTxns in case of reorgs
			delete(oldTxs, tx.Hash())
		}
	}

	// Legacy reorg logic //
	for _, tx := range oldTxs {
		if err := p.addTx(reorg, tx); err != nil {
			p.logger.Error("add tx", "err", err)
		}
	}

	if len(stateNonces) == 0 {
		return
	}

	// reset accounts with the new state
	p.resetAccounts(stateNonces)
}

// validateTx ensures the transaction conforms to specific
// constraints before entering the pool.
func (p *TxPool) validateTx(tx *types.Transaction) error {
	// Check the transaction size to overcome DOS Attacks
	if uint64(len(tx.MarshalRLP())) > txMaxSize {
		return ErrOversizedData
	}

	// Check if the transaction has a strictly positive value
	if tx.Value.Sign() < 0 {
		return ErrNegativeValue
	}

	// Check if the transaction is signed properly

	// Extract the sender
	from, signerErr := p.signer.Sender(tx)
	if signerErr != nil {
		return ErrExtractSignature
	}

	if _, ok := p.blacklist[from]; ok {
		return ErrBlackList
	}

	// If the from field is set, check that
	// it matches the signer
	if tx.From != types.ZeroAddress &&
		tx.From != from {
		return ErrInvalidSender
	}

	// If no address was set, update it
	if tx.From == types.ZeroAddress {
		tx.From = from
	}

	// Reject underpriced transactions
	if tx.IsUnderpriced(p.priceLimit) {
		return ErrUnderpriced
	}

	// Grab the state root for the latest block
	stateRoot := p.store.Header().StateRoot

	// Check nonce ordering
	if p.store.GetNonce(stateRoot, tx.From) > tx.Nonce {
		return ErrNonceTooLow
	}

	accountBalance, balanceErr := p.store.GetBalance(stateRoot, tx.From)
	if balanceErr != nil {
		return ErrInvalidAccountState
	}

	// Check if the sender has enough funds to execute the transaction
	if accountBalance.Cmp(tx.Cost()) < 0 {
		return ErrInsufficientFunds
	}

	// Make sure the transaction has more gas than the basic transaction fee
	intrinsicGas, err := state.TransactionGasCost(tx, p.forks.Homestead, p.forks.Istanbul)
	if err != nil {
		return err
	}

	if tx.Gas < intrinsicGas {
		return ErrIntrinsicGas
	}

	// Grab the block gas limit for the latest block
	latestBlockGasLimit := p.store.Header().GasLimit

	if tx.Gas > latestBlockGasLimit {
		return ErrBlockLimitExceeded
	}

	return nil
}

// addTx is the main entry point to the pool
// for all new transactions. If the call is
// successful, an account is created for this address
// (only once) and an enqueueRequest is signaled.
func (p *TxPool) addTx(origin txOrigin, tx *types.Transaction) error {
	if p.IsDDOSTx(tx) {
		return ErrContractDDOSList
	}

	// get the hash already from the very beginning
	p.logger.Debug("add tx",
		"origin", origin.String(),
		"hash", tx.Hash().String(),
	)

	// validate incoming tx
	if err := p.validateTx(tx); err != nil {
		return err
	}

	// check for overflow
	if p.gauge.read()+slotsRequired(tx) > p.gauge.max {
		return ErrTxPoolOverflow
	}

	// add to index
	if ok := p.index.add(tx); !ok {
		return ErrAlreadyKnown
	}

	if tx.ReceivedTime.IsZero() {
		tx.ReceivedTime = time.Now() // mark the tx received time
	}

	// initialize account for this address once
	if !p.accounts.exists(tx.From) {
		p.createAccountOnce(tx.From)
	}

	// send request [BLOCKING]
	p.enqueueReqCh <- enqueueRequest{tx: tx}
	p.eventManager.signalEvent(proto.EventType_ADDED, tx.Hash())

	return nil
}

// handleEnqueueRequest attempts to enqueue the transaction
// contained in the given request to the associated account.
// If, afterwards, the account is eligible for promotion,
// a promoteRequest is signaled.
func (p *TxPool) handleEnqueueRequest(req enqueueRequest) {
	tx := req.tx
	addr := req.tx.From

	// fetch account
	account := p.accounts.get(addr)

	// enqueue tx
	replacedTx, err := account.enqueue(tx)
	if err != nil {
		p.logger.Error("enqueue request", "err", err)

		// remove it from index when nonce too low
		p.index.remove(tx)

		return
	}

	// old tx exists, replacement
	if replacedTx != nil {
		p.logger.Debug(
			"replace enquque transaction",
			"old",
			replacedTx.Hash().String(),
			"new",
			tx.Hash().String(),
		)

		// remove tx index
		p.index.remove(replacedTx)
		// gauge, metrics, event
		p.gauge.decrease(slotsRequired(replacedTx))
		p.metrics.AddEnqueueTxs(-1)
		p.eventManager.signalEvent(proto.EventType_REPLACED, replacedTx.Hash())
	}

	p.logger.Debug("enqueue request", "hash", tx.Hash())

	// state
	p.gauge.increase(slotsRequired(tx))
	// metrics and event
	p.increaseQueueGauge([]*types.Transaction{tx}, p.metrics.AddEnqueueTxs, proto.EventType_ENQUEUED)

	if tx.Nonce > account.getNonce() {
		// don't signal promotion for
		// higher nonce txs
		return
	}

	p.promoteReqCh <- promoteRequest{account: addr} // BLOCKING
}

// handlePromoteRequest handles moving promotable transactions
// of some account from enqueued to promoted. Can only be
// invoked by handleEnqueueRequest or resetAccount.
func (p *TxPool) handlePromoteRequest(req promoteRequest) {
	addr := req.account
	account := p.accounts.get(addr)

	// promote enqueued txs
	promoted, dropped, replaced := account.promote()
	p.logger.Debug("promote request", "promoted", promoted, "addr", addr.String())

	// drop lower nonce txs first, to reduce the risk of mining.
	if len(dropped) > 0 {
		p.pruneEnqueuedTxs(dropped)
		p.logger.Debug("dropped transactions when promoting", "dropped", dropped)
	}

	if len(replaced) > 0 {
		p.index.remove(replaced...)
		// state
		p.gauge.decrease(slotsRequired(replaced...))
		// metrics and event
		p.decreaseQueueGauge(replaced, p.metrics.AddPendingTxs, proto.EventType_REPLACED)
		p.logger.Debug("replaced transactions when promoting", "replaced", replaced)
	}

	// metrics and event
	p.tranferQueueGauge(promoted, p.metrics.AddEnqueueTxs, p.metrics.AddPendingTxs, proto.EventType_PROMOTED)
}

// pruneStaleAccounts would find out all need-to-prune transactions,
// remove them from txpool.
func (p *TxPool) pruneStaleAccounts() {
	pruned := p.accounts.pruneStaleEnqueuedTxs(p.promoteOutdateDuration)
	if len(pruned) == 0 {
		return
	}

	p.pruneEnqueuedTxs(pruned)
	p.logger.Debug("pruned stale enqueued txs", "num", pruned)
}

func (p *TxPool) tranferQueueGauge(
	txs []*types.Transaction,
	enqueueGaugeAddFn, pendingGaugeAddFn func(v float64),
	event proto.EventType,
) {
	// metrics switching
	enqueueGaugeAddFn(-1 * float64(len(txs)))
	pendingGaugeAddFn(float64(len(txs)))

	// event
	p.eventManager.signalEvent(event, toHash(txs...)...)
}

func (p *TxPool) increaseQueueGauge(txs []*types.Transaction, gaugeAddFn func(v float64), event proto.EventType) {
	// metrics
	gaugeAddFn(float64(len(txs)))
	// event
	p.eventManager.signalEvent(event, toHash(txs...)...)
}

func (p *TxPool) decreaseQueueGauge(txs []*types.Transaction, gaugeAddFn func(v float64), event proto.EventType) {
	// metrics
	gaugeAddFn(-1 * float64(len(txs)))
	// event
	p.eventManager.signalEvent(event, toHash(txs...)...)
}

func (p *TxPool) pruneEnqueuedTxs(pruned []*types.Transaction) {
	p.index.remove(pruned...)
	// state
	p.gauge.decrease(slotsRequired(pruned...))
	// metrics and event
	p.decreaseQueueGauge(pruned, p.metrics.AddEnqueueTxs, proto.EventType_PRUNED_ENQUEUED)
}

// addGossipTx handles receiving transactions gossiped by the network.
func (p *TxPool) addGossipTx(obj interface{}, from peer.ID) {
	if p.isClosed.Load() {
		p.logger.Error("txpool is Closed")

		return
	}

	if !p.getSealing() {
		// we're not validator, not interested in it
		return
	}

	raw, ok := obj.(*proto.Txn)
	if !ok {
		p.logger.Warn("gossip tx(%+v) is not a transaction", obj)

		return
	}

	if raw.Raw == nil || len(raw.Raw.Value) == 0 {
		p.logger.Info("gossip tx raw data is empty")

		return
	}

	tx := new(types.Transaction)

	// decode tx
	if err := tx.UnmarshalRLP(raw.Raw.Value); err != nil {
		p.logger.Error("failed to decode broadcast tx", "err", err)

		return
	}

	// add tx
	if err := p.addTx(gossip, tx); err != nil {
		if errors.Is(err, ErrAlreadyKnown) {
			p.logger.Debug("rejecting known tx (gossip)", "hash", tx.Hash())

			return
		}

		p.logger.Error("failed to add broadcast tx", "err", err, "hash", tx.Hash())
	}
}

// resetAccounts updates existing accounts with the new nonce and prunes stale transactions.
func (p *TxPool) resetAccounts(stateNonces map[types.Address]uint64) {
	var (
		allPrunedPromoted []*types.Transaction
		allPrunedEnqueued []*types.Transaction
	)

	//	clear all accounts of stale txs
	for addr, newNonce := range stateNonces {
		if !p.accounts.exists(addr) {
			// no updates for this account
			continue
		}

		account := p.accounts.get(addr)
		prunedPromoted, prunedEnqueued := account.reset(newNonce, p.promoteReqCh)

		//	append pruned
		allPrunedPromoted = append(allPrunedPromoted, prunedPromoted...)
		allPrunedEnqueued = append(allPrunedEnqueued, prunedEnqueued...)
	}

	//	pool cleanup callback
	cleanup := func(stale ...*types.Transaction) {
		p.index.remove(stale...)
		p.gauge.decrease(slotsRequired(stale...))
	}

	//	prune pool state
	if len(allPrunedPromoted) > 0 {
		cleanup(allPrunedPromoted...)
		p.decreaseQueueGauge(allPrunedPromoted, p.metrics.AddPendingTxs, proto.EventType_PRUNED_PROMOTED)
	}

	if len(allPrunedEnqueued) > 0 {
		cleanup(allPrunedEnqueued...)
		p.decreaseQueueGauge(allPrunedEnqueued, p.metrics.AddEnqueueTxs, proto.EventType_PRUNED_ENQUEUED)
	}
}

// createAccountOnce creates an account and
// ensures it is only initialized once.
func (p *TxPool) createAccountOnce(newAddr types.Address) *account {
	// fetch nonce from state
	stateRoot := p.store.Header().StateRoot
	stateNonce := p.store.GetNonce(stateRoot, newAddr)

	// initialize the account
	account := p.accounts.initOnce(newAddr, stateNonce)

	return account
}

// Length returns the total number of all promoted transactions.
func (p *TxPool) Length() uint64 {
	return p.accounts.promoted()
}

// toHash returns the hash(es) of given transaction(s)
func toHash(txs ...*types.Transaction) (hashes []types.Hash) {
	for _, tx := range txs {
		hashes = append(hashes, tx.Hash())
	}

	return
}
