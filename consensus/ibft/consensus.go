package ibft

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/dogechain-lab/dogechain/consensus/ibft/currentstate"
	"github.com/dogechain-lab/dogechain/consensus/ibft/proto"
	"github.com/dogechain-lab/dogechain/types"
)

// runSequence starts the underlying consensus mechanism for the given height.
// It may be called by a single thread at any given time
func (i *Ibft) runSequence(height uint64) <-chan struct{} {
	done := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background()) // never stop until cancel

	i.cancelSequence = cancel

	i.wg.Add(1)

	go func() {
		defer func() {
			cancel()
			i.wg.Done()
			close(done)
		}()

		i.runSequenceAtHeight(ctx, height)
	}()

	return done
}

// stopSequence terminates the running IBFT sequence gracefully and waits for it to return
func (i *Ibft) stopSequence() {
	if i.cancelSequence != nil {
		i.cancelSequence()

		i.cancelSequence = nil

		i.wg.Wait()
	}
}

func (i *Ibft) runSequenceAtHeight(ctx context.Context, height uint64) {
	// Set the starting state data
	i.state.Clear(height)
	i.msgQueue.PruneByHeight(height)

	i.logger.Info("sequence started", "height", height)
	defer i.logger.Info("sequence done", "height", height)

	for {
		select {
		case <-i.closeCh:
			return
		default:
		}

		if done := i.runCycle(ctx); done {
			return
		}
	}
}

// runCycle represents the IBFT state machine loop
func (i *Ibft) runCycle(ctx context.Context) (shouldStop bool) {
	// Log to the console
	i.logger.Debug("cycle",
		"state", i.getState(),
		"sequence", i.state.Sequence(),
		"round", i.state.Round()+1,
	)

	select {
	case <-ctx.Done():
		i.logger.Debug("sequence cancelled")

		return true
	default: // go on
	}

	// Based on the current state, execute the corresponding section
	switch i.getState() {
	case currentstate.AcceptState:
		i.runAcceptState(ctx)

	case currentstate.ValidateState:
		i.runValidateState(ctx)

	case currentstate.RoundChangeState:
		i.runRoundChangeState(ctx)

	case currentstate.CommitState:
		i.runCommitState(ctx)

	case currentstate.FinState:
		return true
	}

	return false
}

// getState returns the current IBFT state
func (i *Ibft) getState() currentstate.IbftState {
	return i.state.GetState()
}

// isState checks if the node is in the passed in state
func (i *Ibft) isState(s currentstate.IbftState) bool {
	return i.state.GetState() == s
}

// setState sets the IBFT state
func (i *Ibft) setState(s currentstate.IbftState) {
	i.logger.Info("state change", "new", s)
	i.state.SetState(s)
}

// runAcceptState runs the Accept state loop
//
// The Accept state always checks the snapshot, and the validator set. If the current node is not in the validators set,
// it moves back to the Sync state. On the other hand, if the node is a validator, it calculates the proposer.
// If it turns out that the current node is the proposer, it builds a block,
// and sends preprepare and then prepare messages.
func (i *Ibft) runAcceptState(ctx context.Context) (shouldStop bool) { // start new round
	if i.isClosed.Load() {
		return
	}

	// set log output
	logger := i.logger.Named("acceptState")
	logger.Info("Accept state", "sequence", i.state.Sequence(), "round", i.state.Round()+1)
	// set consensus_rounds metric output
	i.metrics.Rounds.Set(float64(i.state.Round() + 1))

	// This is the state in which we either propose a block or wait for the pre-prepare message
	parent := i.blockchain.Header()
	number := parent.Number + 1

	if number != i.state.Sequence() {
		logger.Error("sequence not correct", "parent", parent.Number, "sequence", i.state.Sequence())
		time.Sleep(1 * time.Second)

		return
	}

	// update current module cache
	if err := i.updateCurrentModules(number); err != nil {
		logger.Error(
			"failed to update submodules",
			"height", number,
			"err", err,
		)
	}

	snap, err := i.getSnapshot(parent.Number)

	if err != nil {
		logger.Error("cannot find snapshot", "num", parent.Number)
		time.Sleep(1 * time.Second)

		return
	}

	if !snap.Set.Includes(i.validatorKeyAddr) {
		// we are not a validator anymore, move back to sync state
		logger.Info("we are not a validator anymore")
		time.Sleep(1 * time.Second)

		return
	}

	if hookErr := i.runHook(AcceptStateLogHook, i.state.Sequence(), snap); hookErr != nil {
		logger.Error(fmt.Sprintf("Unable to run hook %s, %v", AcceptStateLogHook, hookErr))
	}

	i.state.SetValidators(snap.Set)

	//Update the No.of validator metric
	i.metrics.Validators.Set(float64(len(snap.Set)))
	// reset round messages
	i.state.ResetRoundMsgs()

	// select the proposer of the block
	var lastProposer types.Address
	if parent.Number != 0 {
		lastProposer, _ = ecrecoverFromHeader(parent)
	}

	if hookErr := i.runHook(CalculateProposerHook, i.state.Sequence(), lastProposer); hookErr != nil {
		logger.Error(fmt.Sprintf("Unable to run hook %s, %v", CalculateProposerHook, hookErr))
	}

	if i.state.Proposer() == i.validatorKeyAddr {
		logger.Info("we are the proposer", "block", number)

		if !i.state.IsLocked() {
			// since the state is not locked, we need to build a new block
			block, err := i.buildBlock(snap, parent)
			if err != nil {
				logger.Error("failed to build block", "err", err)
				i.setState(currentstate.RoundChangeState)

				return
			}

			i.state.SetBlock(block)

			// calculate how much time do we have to wait to mine the block
			delay := time.Until(time.Unix(int64(block.Header.Timestamp), 0))

			delayTimer := time.NewTimer(delay)
			defer delayTimer.Stop()

			select {
			case <-delayTimer.C:
			case <-i.closeCh:
				return
			}
		}

		// send the preprepare message as an RLP encoded block
		i.sendPreprepareMsg()

		// send the prepare message since we are ready to move the state
		i.sendPrepareMsg()

		// move to validation state for new prepare messages
		i.setState(currentstate.ValidateState)

		return
	}

	logger.Info("proposer calculated", "proposer", i.state.Proposer(), "block", number)

	// we are NOT a proposer for the block. Then, we have to wait
	// for a pre-prepare message from the proposer

	timeout := i.state.MessageTimeout()
	for i.getState() == currentstate.AcceptState {
		msg, continuable := i.getNextMessage(ctx, timeout)
		if !continuable {
			return true
		}

		if msg == nil {
			i.setState(currentstate.RoundChangeState)

			continue
		}

		if msg.From != i.state.Proposer().String() {
			logger.Error("msg received from wrong proposer")

			continue
		}

		if msg.Proposal == nil {
			// A malicious node conducted a DoS attack
			logger.Error("proposal data in msg is nil")

			continue
		}

		// retrieve the block proposal
		block := &types.Block{}
		if err := block.UnmarshalRLP(msg.Proposal.Value); err != nil {
			logger.Error("failed to unmarshal block", "err", err)
			i.setState(currentstate.RoundChangeState)

			return
		}

		// Make sure the proposing block height match the current sequence
		if block.Number() != i.state.Sequence() {
			logger.Error("sequence not correct", "block", block.Number, "sequence", i.state.Sequence())
			i.handleStateErr(errIncorrectBlockHeight)

			return
		}

		if i.state.IsLocked() {
			// the state is locked, we need to receive the same block
			if block.Hash() == i.state.Block().Hash() {
				// fast-track and send a commit message and wait for validations
				i.sendCommitMsg()
				i.setState(currentstate.ValidateState)
			} else {
				i.handleStateErr(errIncorrectBlockLocked)
			}
		} else {
			// since it's a new block, we have to verify it first
			if err := i.verifyHeaderImpl(snap, parent, block.Header); err != nil {
				logger.Error("block header verification failed", "err", err)
				i.handleStateErr(errBlockVerificationFailed)

				continue
			}

			// Verify other block params
			if err := i.blockchain.VerifyPotentialBlock(block); err != nil {
				logger.Error("block verification failed", "err", err)
				i.handleStateErr(errBlockVerificationFailed)

				continue
			}

			if hookErr := i.runHook(VerifyBlockHook, block.Number(), block); hookErr != nil {
				if errors.Is(hookErr, errBlockVerificationFailed) {
					logger.Error("block verification failed, block at the end of epoch has transactions")
					i.handleStateErr(errBlockVerificationFailed)
				} else {
					logger.Error(fmt.Sprintf("Unable to run hook %s, %v", VerifyBlockHook, hookErr))
				}

				continue
			}

			i.state.SetBlock(block)
			// send prepare message and wait for validations
			i.sendPrepareMsg()
			i.setState(currentstate.ValidateState)
		}
	}

	return false
}

// runValidateState implements the Validate state loop.
//
// The Validate state is rather simple - all nodes do in this state is read messages
// and add them to their local snapshot state
func (i *Ibft) runValidateState(ctx context.Context) (shouldStop bool) {
	logger := i.logger.Named("validateState")

	// for all validators commit checking
	hasCommitted := false
	sendCommit := func() {
		if hasCommitted {
			return
		}
		// at this point either we have enough prepare messages
		// or commit messages so we can lock the block
		i.state.Lock()
		// send the commit message
		i.sendCommitMsg()

		hasCommitted = true
	}
	// change round logic
	changeRound := func() {
		i.state.Unlock()
		i.setState(currentstate.RoundChangeState)
	}
	// for proposer post commit checking
	hasPostCommitted := false
	// send post commit logic to check without
	sendPostCommit := func() {
		if hasPostCommitted {
			return
		}

		// update flag for repeating skip
		hasPostCommitted = true
		// only proposer need to send post commit
		signer, _ := ecrecoverFromHeader(i.state.Block().Header)
		if signer == i.validatorKeyAddr {
			i.sendPostCommitMsg()
		}
	}

	for i.getState() == currentstate.ValidateState {
		timeout := i.state.MessageTimeout()

		msg, continuable := i.getNextMessage(ctx, timeout)
		if !continuable {
			return true
		}

		if msg == nil {
			logger.Info("ValidateState got message timeout, should change round",
				"sequence", i.state.Sequence(), "round", i.state.Round()+1)
			changeRound()

			return
		}

		if msg.View == nil {
			// A malicious node conducted a DoS attack
			logger.Error("view data in msg is nil")

			continue
		}

		// check msg number and round, might from some faulty nodes
		if i.state.Sequence() != msg.View.GetSequence() ||
			i.state.Round() != msg.View.GetRound() {
			logger.Debug("ValidateState got message not matching sequence and round",
				"my-sequence", i.state.Sequence(), "my-round", i.state.Round()+1,
				"other-sequence", msg.View.GetSequence(), "other-round", msg.View.GetRound())

			continue
		}

		currentBlockHash := i.state.Block().Hash()

		switch msg.Type {
		case proto.MessageReq_Prepare:
			i.state.AddPrepared(msg)

		case proto.MessageReq_Commit:
			// check seal before execute any of it
			_, addr, err := committedSealFromHex(msg.Seal, currentBlockHash)
			if err != nil ||
				addr != msg.FromAddr() {
				logger.Warn("invalid seal",
					"blockHash", currentBlockHash,
					"msg", msg,
					"signer", addr,
					"err", err,
				)

				break // current switch
			}

			i.state.AddCommitted(msg)

		case proto.MessageReq_PostCommit:
			// not valid canonical seals
			if msg.Canonical == nil ||
				msg.Canonical.Hash != currentBlockHash.String() ||
				len(msg.Canonical.Seals) < i.state.NumValid() {
				logger.Error("invalid canonical seal",
					"blockHash", i.state.Block().Hash(),
					"msg", msg,
				)
				changeRound()

				return
			}

			i.state.AddPostCommitted(msg)

		default:
			logger.Error("BUG: %s, validate state do not handle type.msg: %d",
				reflect.TypeOf(msg.Type), msg.Type)
		}

		if i.state.NumPrepared() > i.state.NumValid() {
			// we have received enough pre-prepare messages
			sendCommit()
		}

		if i.state.NumCommitted() > i.state.NumValid() {
			// send post commit message
			sendPostCommit()
		}

		if i.state.CanonicalSeal() != nil {
			logger.Info("got canonical seal and move on")
			// switch to commit state
			i.setState(currentstate.CommitState)
			// get out of loop
			break
		}
	}

	return
}

func (i *Ibft) runCommitState(ctx context.Context) (shouldStop bool) {
	switch i.getState() {
	case currentstate.CommitState:
	default:
		return
	}

	// at this point either if it works or not we need to unlock
	block := i.state.Block()
	i.state.Unlock()

	if err := i.insertBlock(block); err != nil {
		// start a new round with the state unlocked since we need to
		// be able to propose/validate a different block
		i.logger.Named("commitState").Error("failed to insert block", "err", err)
		i.handleStateErr(errFailedToInsertBlock)

		return
	}

	// update metrics
	i.updateMetrics(block)

	// move ahead to fin state
	i.setState(currentstate.FinState)

	return
}

func (i *Ibft) runRoundChangeState(ctx context.Context) (shouldStop bool) {
	logger := i.logger.Named("roundChangeState")

	sendRoundChange := func(round uint64) {
		logger.Debug("local round change", "round", round+1)
		// set the new round and update the round metric
		i.startNewRound(round)
		i.metrics.Rounds.Set(float64(round))
		// clean the round
		i.state.CleanRound(round)
		// send the round change message
		i.sendRoundChange()
	}
	sendNextRoundChange := func() {
		sendRoundChange(i.state.NextRound())
	}

	checkTimeout := func() {
		// check if there is any peer that is really advanced and we might need to sync with it first
		if i.syncer != nil && i.syncer.HasSyncPeer() {
			i.setState(currentstate.AcceptState)

			return
		}

		// otherwise, it seems that we are in sync
		// and we should start a new round
		sendNextRoundChange()
	}

	// if the round was triggered due to an error, we send our own
	// next round change
	if err := i.state.ConsumeErr(); err != nil {
		logger.Debug("round change handle err", "err", err)
		sendNextRoundChange()
	} else {
		// otherwise, it is due to a timeout in any stage
		// First, we try to sync up with any max round already available
		if maxRound, ok := i.state.MaxRound(); ok {
			logger.Debug("round change set max round", "round", maxRound)
			sendRoundChange(maxRound)
		} else {
			// otherwise, do your best to sync up
			checkTimeout()
		}
	}

	// create a timer for the round change
	for i.getState() == currentstate.RoundChangeState {
		// timeout should update every time it enters a new round
		timeout := i.state.MessageTimeout()

		msg, continuable := i.getNextMessage(ctx, timeout)
		if !continuable {
			return true
		}

		if msg == nil {
			logger.Info("round change timeout")
			checkTimeout()

			continue
		}

		if msg.View == nil {
			// A malicious node conducted a DoS attack
			logger.Error("view data in msg is nil")

			continue
		}

		// we only expect RoundChange messages right now
		num := i.state.AddRoundMessage(msg)

		if num >= i.state.NumValid() {
			// start a new round immediately
			i.startNewRound(msg.View.Round)
			i.setState(currentstate.AcceptState)
		} else if num == i.state.MaxFaultyNodes()+1 {
			// weak certificate, try to catch up if our round number is smaller
			if i.state.Round() < msg.View.Round {
				sendRoundChange(msg.View.Round)
			}
		}
	}

	return
}
