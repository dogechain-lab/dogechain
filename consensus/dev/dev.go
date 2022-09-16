package dev

import (
	"context"
	"fmt"
	"time"

	"github.com/dogechain-lab/dogechain/blockchain"
	"github.com/dogechain-lab/dogechain/consensus"
	"github.com/dogechain-lab/dogechain/contracts/upgrader"
	"github.com/dogechain-lab/dogechain/helper/progress"
	"github.com/dogechain-lab/dogechain/state"
	"github.com/dogechain-lab/dogechain/txpool"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/hashicorp/go-hclog"
)

// Dev consensus protocol seals any new transaction immediately
type Dev struct {
	logger hclog.Logger

	notifyCh chan struct{}
	closeCh  chan struct{}

	interval uint64
	txpool   *txpool.TxPool

	blockchain *blockchain.Blockchain
	executor   *state.Executor
}

// Factory implements the base factory method
func Factory(
	params *consensus.ConsensusParams,
) (consensus.Consensus, error) {
	logger := params.Logger.Named("dev")

	d := &Dev{
		logger:     logger,
		notifyCh:   make(chan struct{}),
		closeCh:    make(chan struct{}),
		blockchain: params.Blockchain,
		executor:   params.Executor,
		txpool:     params.Txpool,
	}

	rawInterval, ok := params.Config.Config["interval"]
	if ok {
		interval, ok := rawInterval.(uint64)
		if !ok {
			return nil, fmt.Errorf("interval expected int")
		}

		d.interval = interval
	}

	return d, nil
}

// Initialize initializes the consensus
func (d *Dev) Initialize() error {
	return nil
}

// Start starts the consensus mechanism
func (d *Dev) Start() error {
	go d.run()

	return nil
}

func (d *Dev) nextNotify() chan struct{} {
	if d.interval == 0 {
		d.interval = 1
	}

	delay := time.NewTimer(time.Duration(d.interval) * time.Second)

	go func() {
		<-delay.C
		d.notifyCh <- struct{}{}
	}()

	return d.notifyCh
}

func (d *Dev) run() {
	d.logger.Info("consensus started")

	for {
		// wait until there is a new txn
		select {
		case <-d.nextNotify():
		case <-d.closeCh:
			return
		}

		// There are new transactions in the pool, try to seal them
		header := d.blockchain.Header()
		if err := d.writeNewBlock(header); err != nil {
			d.logger.Error("failed to mine block", "err", err)
		}
	}
}

type transitionInterface interface {
	Write(txn *types.Transaction) error
}

func (d *Dev) writeTransactions(gasLimit uint64, transition transitionInterface) []*types.Transaction {
	var includedTxs []*types.Transaction

	// get all pending transactions once and for all
	pendingTxs := d.txpool.Pending()
	// get highest price transaction queue
	priceTxs := types.NewTransactionsByPriceAndNonce(pendingTxs)

	for {
		tx := priceTxs.Peek()
		if tx == nil {
			d.logger.Debug("no more transactions")

			break
		}

		if tx.ExceedsBlockGasLimit(gasLimit) {
			// The address is punished. For current loop, it would not include its transactions any more.
			d.txpool.Drop(tx)
			priceTxs.Pop()

			continue
		}

		if err := transition.Write(tx); err != nil {
			//nolint:errorlint
			if _, ok := err.(*state.AllGasUsedError); ok {
				// no more transaction could be packed
				d.logger.Debug("Not enough gas for further transactions")

				break
			} else if _, ok := err.(*state.GasLimitReachedTransitionApplicationError); ok {
				// Ignore transaction when the free gas not enough
				d.logger.Debug("Gas limit exceeded for current block", "from", tx.From)
				priceTxs.Pop()
			} else if nonceErr, ok := err.(*state.NonceTooLowError); ok {
				// low nonce tx, should reset accounts once done
				d.logger.Warn("write transaction nonce too low",
					"hash", tx.Hash, "from", tx.From, "nonce", tx.Nonce)
				// skip the address, whose txs should be reset first.
				d.txpool.DemoteAllPromoted(tx, nonceErr.CorrectNonce)
				priceTxs.Pop()
			} else if nonceErr, ok := err.(*state.NonceTooHighError); ok {
				// high nonce tx, should reset accounts once done
				d.logger.Error("write miss some transactions with higher nonce",
					tx.Hash, "from", tx.From, "nonce", tx.Nonce)
				d.txpool.DemoteAllPromoted(tx, nonceErr.CorrectNonce)
				priceTxs.Pop()
			} else {
				// no matter what kind of failure, drop is reasonable for not executed it yet
				d.logger.Debug("write not executed transaction failed",
					"hash", tx.Hash, "from", tx.From,
					"nonce", tx.Nonce, "err", err)
				d.txpool.Drop(tx)
				priceTxs.Pop()
			}

			continue
		}

		// no errors, go on
		priceTxs.Shift()

		includedTxs = append(includedTxs, tx)
	}

	d.logger.Info("picked out txns from pool", "num", len(includedTxs))

	return includedTxs
}

// writeNewBLock generates a new block based on transactions from the pool,
// and writes them to the blockchain
func (d *Dev) writeNewBlock(parent *types.Header) error {
	// Generate the base block
	num := parent.Number
	header := &types.Header{
		ParentHash: parent.Hash,
		Number:     num + 1,
		GasLimit:   parent.GasLimit, // Inherit from parent for now, will need to adjust dynamically later.
		Timestamp:  uint64(time.Now().Unix()),
	}

	// calculate gas limit based on parent header
	gasLimit, err := d.blockchain.CalculateGasLimit(header.Number)
	if err != nil {
		return err
	}

	header.GasLimit = gasLimit

	miner, err := d.GetBlockCreator(header)
	if err != nil {
		return err
	}

	transition, err := d.executor.BeginTxn(parent.StateRoot, header, miner)

	if err != nil {
		return err
	}

	txns := d.writeTransactions(gasLimit, transition)

	// upgrade system if needed
	upgrader.UpgradeSystem(
		d.blockchain.Config().ChainID,
		d.blockchain.Config().Forks,
		header.Number,
		transition.Txn(),
		d.logger,
	)

	// Commit the changes
	_, root := transition.Commit()

	// Update the header
	header.StateRoot = root
	header.GasUsed = transition.TotalGas()

	// Build the actual block
	// The header hash is computed inside buildBlock
	block := consensus.BuildBlock(consensus.BuildBlockParams{
		Header:   header,
		Txns:     txns,
		Receipts: transition.Receipts(),
	})

	if err := d.blockchain.VerifyFinalizedBlock(block); err != nil {
		return err
	}

	// Write the block to the blockchain
	if err := d.blockchain.WriteBlock(block); err != nil {
		return err
	}

	// after the block has been written we reset the txpool so that
	// the old transactions are removed
	d.txpool.ResetWithHeaders(block.Header)

	return nil
}

// REQUIRED BASE INTERFACE METHODS //

func (d *Dev) VerifyHeader(header *types.Header) error {
	// All blocks are valid
	return nil
}

func (d *Dev) ProcessHeaders(headers []*types.Header) error {
	return nil
}

func (d *Dev) GetBlockCreator(header *types.Header) (types.Address, error) {
	return header.Miner, nil
}

// PreStateCommit a hook to be called before finalizing state transition on inserting block
func (d *Dev) PreStateCommit(_header *types.Header, _txn *state.Transition) error {
	return nil
}

func (d *Dev) GetSyncProgression() *progress.Progression {
	return nil
}

func (d *Dev) Prepare(header *types.Header) error {
	// TODO: Remove
	return nil
}

func (d *Dev) Seal(block *types.Block, ctx context.Context) (*types.Block, error) {
	// TODO: Remove
	return nil, nil
}

func (d *Dev) Close() error {
	close(d.closeCh)

	return nil
}
