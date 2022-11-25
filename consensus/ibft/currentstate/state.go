package currentstate

import (
	"fmt"
	"math"
	"sync/atomic"
	"time"

	"github.com/dogechain-lab/dogechain/consensus/ibft/proto"
	"github.com/dogechain-lab/dogechain/consensus/ibft/validator"
	"github.com/dogechain-lab/dogechain/types"
)

type IbftState uint32

// Define the states in IBFT
const (
	AcceptState IbftState = iota
	RoundChangeState
	ValidateState // including prepare, commit, and post commit state stage
	CommitState   // committed
	FinState      // finish current sequence
)

// String returns the string representation of the passed in state
func (i IbftState) String() string {
	switch i {
	case AcceptState:
		return "AcceptState"

	case RoundChangeState:
		return "RoundChangeState"

	case ValidateState:
		return "ValidateState"

	case CommitState:
		return "CommitState"

	case FinState:
		return "FinState"
	}

	panic(fmt.Sprintf("BUG: Ibft state not found %d", i))
}

// CurrentState defines the current state object in IBFT
//
// NOTE: not thread safe, better to wrap it in mutext for concurrent use case
type CurrentState struct {
	// validators represent the current validator set
	validators validator.Validators

	// state is the current state
	state uint64

	// The proposed block
	block *types.Block

	// The selected proposer
	proposer types.Address

	// Current view
	view *proto.View

	// additionalTimeout is for block building, which might consumes longer than we thought
	additionalTimeout time.Duration

	// List of prepared messages
	prepared map[types.Address]*proto.MessageReq

	// List of committed messages
	committed map[types.Address]*proto.MessageReq

	// canonical seal from proposer
	canonicalSeal *proto.MessageReq

	// List of round change messages
	roundMessages map[uint64]map[types.Address]*proto.MessageReq

	// Locked signals whether the proposal is locked
	locked bool

	// Describes whether there has been an error during the computation
	err error
}

// NewState creates a new state with reset round messages
func NewState() *CurrentState {
	c := &CurrentState{}
	c.ResetRoundMsgs()

	return c
}

func (c *CurrentState) Clear(height uint64) {
	c.SetState(AcceptState)
	c.block = nil
	c.proposer = types.ZeroAddress
	c.view = &proto.View{
		Sequence: height,
		Round:    0,
	}
	c.err = nil
	c.prepared = map[types.Address]*proto.MessageReq{}
	c.committed = map[types.Address]*proto.MessageReq{}
	c.canonicalSeal = nil // reset canonical seal when round change
	c.roundMessages = map[uint64]map[types.Address]*proto.MessageReq{}
	c.locked = false
}

// GetState returns the current state
func (c *CurrentState) GetState() IbftState {
	stateAddr := &c.state

	return IbftState(atomic.LoadUint64(stateAddr))
}

// SetState sets the current state
func (c *CurrentState) SetState(s IbftState) {
	stateAddr := &c.state

	atomic.StoreUint64(stateAddr, uint64(s))
}

func (c *CurrentState) SetBlock(b *types.Block) {
	c.block = b
}

func (c *CurrentState) Block() *types.Block {
	return c.block
}

func (c *CurrentState) SetValidators(validators []types.Address) {
	c.validators = validators
}

func (c *CurrentState) Validators() []types.Address {
	return c.validators
}

// NumValid returns the number of required messages
func (c *CurrentState) NumValid() int {
	// According to the IBFT spec, the number of valid messages
	// needs to be 2F + 1
	// The 1 missing from this equation is accounted for elsewhere
	// (the current node tallying the messages will include its own message)
	return 2 * c.MaxFaultyNodes()
}

func (c *CurrentState) MaxFaultyNodes() int {
	return validator.CalcMaxFaultyNodes(c.validators)
}

func (c *CurrentState) HandleErr(err error) {
	c.err = err
}

// ConsumeErr returns the current error, if any, and consumes it
func (c *CurrentState) ConsumeErr() error {
	err := c.err
	c.err = nil

	return err
}

func (c *CurrentState) PeekError() error {
	return c.err
}

func (c *CurrentState) Sequence() uint64 {
	if c.view != nil {
		return c.view.Sequence
	}

	return 0
}

func (c *CurrentState) Round() uint64 {
	if c.view != nil {
		return c.view.Round
	}

	return 0
}

func (c *CurrentState) NextRound() uint64 {
	return c.view.Round + 1
}

func (c *CurrentState) MaxRound() (maxRound uint64, found bool) {
	num := validator.CalcMaxFaultyNodes(c.validators) + 1

	for k, round := range c.roundMessages {
		if len(round) < num {
			continue
		}

		if maxRound < k {
			maxRound = k
			found = true
		}
	}

	return
}

const (
	baseTimeout = 10 * time.Second
	maxTimeout  = 300 * time.Second
)

// MessageTimeout returns duration for waiting message
//
// Consider the network travel time between most validators, using validator
// numbers instead of rounds.
func (c *CurrentState) MessageTimeout() time.Duration {
	if c.Round() >= 8 {
		return maxTimeout
	}

	timeout := baseTimeout
	if c.Round() > 0 {
		timeout += time.Duration(int64(math.Pow(2, float64(c.Round())))) * time.Second
	}

	// add more time for long time block production
	return timeout + c.additionalTimeout
}

// ResetRoundMsgs resets the prepared, committed and round messages in the current state
func (c *CurrentState) ResetRoundMsgs() {
	c.prepared = map[types.Address]*proto.MessageReq{}
	c.committed = map[types.Address]*proto.MessageReq{}
	c.canonicalSeal = nil // reset canonical seal when round change
	c.roundMessages = map[uint64]map[types.Address]*proto.MessageReq{}
}

// CalcProposer calculates the proposer and sets it to the state
func (c *CurrentState) CalcProposer(lastProposer types.Address) {
	c.proposer = c.validators.CalcProposer(c.view.Round, lastProposer)
}

func (c *CurrentState) Proposer() types.Address {
	return c.proposer
}

func (c *CurrentState) CalcNeedPunished(
	currentRound uint64,
	lastBlockProposer types.Address,
) (addr []types.Address) {
	if currentRound == 0 {
		// no one need to be punished
		return nil
	}

	// only punish the first validator,
	p := c.validators.CalcProposer(0, lastBlockProposer)
	addr = append(addr, p)

	return addr
}

func (c *CurrentState) SetView(view *proto.View) {
	c.view = view
}

func (c *CurrentState) View() *proto.View {
	return c.view
}

// SetAdditionalTimeout sets an additional timeout for potentially long builds
func (c *CurrentState) SetAdditionalTimeout(d time.Duration) {
	c.additionalTimeout = d
}

func (c *CurrentState) IsLocked() bool {
	return c.locked
}

func (c *CurrentState) Lock() {
	c.locked = true
}

func (c *CurrentState) Unlock() {
	c.block = nil
	c.locked = false
}

// CleanRound deletes the specific round messages
func (c *CurrentState) CleanRound(round uint64) {
	delete(c.roundMessages, round)
}

// AddRoundMessage adds a message to the round, and returns the round message size
func (c *CurrentState) AddRoundMessage(msg *proto.MessageReq) int {
	if msg.Type != proto.MessageReq_RoundChange {
		return 0
	}

	c.AddMessage(msg)

	return len(c.roundMessages[msg.View.Round])
}

// AddPrepared adds a prepared message
func (c *CurrentState) AddPrepared(msg *proto.MessageReq) {
	if msg.Type != proto.MessageReq_Prepare {
		return
	}

	c.AddMessage(msg)
}

// AddCommitted adds a committed message
func (c *CurrentState) AddCommitted(msg *proto.MessageReq) {
	if msg.Type != proto.MessageReq_Commit {
		return
	}

	c.AddMessage(msg)
}

func (c *CurrentState) AddPostCommitted(msg *proto.MessageReq) {
	if msg.Type != proto.MessageReq_PostCommit {
		return
	}

	c.AddMessage(msg)
}

func (c *CurrentState) Committed() map[types.Address]*proto.MessageReq {
	return c.committed
}

// AddMessage adds a new message to one of the following message lists: committed, prepared, roundMessages
func (c *CurrentState) AddMessage(msg *proto.MessageReq) {
	addr := msg.FromAddr()
	if !c.validators.Includes(addr) {
		// only include messages from validators
		return
	}

	switch msg.Type {
	case proto.MessageReq_PostCommit:
		if c.proposer == addr && msg != nil {
			c.canonicalSeal = msg
		}
	case proto.MessageReq_Commit:
		c.committed[addr] = msg
	case proto.MessageReq_Prepare:
		c.prepared[addr] = msg
	case proto.MessageReq_RoundChange:
		view := msg.View
		if _, ok := c.roundMessages[view.Round]; !ok {
			c.roundMessages[view.Round] = map[types.Address]*proto.MessageReq{}
		}

		c.roundMessages[view.Round][addr] = msg
	}
}

// NumPrepared returns the number of messages in the prepared message list
func (c *CurrentState) NumPrepared() int {
	return len(c.prepared)
}

// numCommitted returns the number of messages in the committed message list
func (c *CurrentState) NumCommitted() int {
	return len(c.committed)
}

func (c *CurrentState) CanonicalSeal() *proto.MessageReq {
	return c.canonicalSeal
}
