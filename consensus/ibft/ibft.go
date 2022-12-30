package ibft

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dogechain-lab/dogechain/blockchain"
	"github.com/dogechain-lab/dogechain/consensus"
	"github.com/dogechain-lab/dogechain/consensus/ibft/currentstate"
	"github.com/dogechain-lab/dogechain/consensus/ibft/proto"
	"github.com/dogechain-lab/dogechain/consensus/ibft/validator"
	"github.com/dogechain-lab/dogechain/contracts/systemcontracts"
	"github.com/dogechain-lab/dogechain/contracts/upgrader"
	"github.com/dogechain-lab/dogechain/contracts/validatorset"
	"github.com/dogechain-lab/dogechain/crypto"
	"github.com/dogechain-lab/dogechain/helper/common"
	"github.com/dogechain-lab/dogechain/helper/hex"
	"github.com/dogechain-lab/dogechain/helper/progress"
	"github.com/dogechain-lab/dogechain/network"
	"github.com/dogechain-lab/dogechain/protocol"
	"github.com/dogechain-lab/dogechain/secrets"
	"github.com/dogechain-lab/dogechain/state"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/hashicorp/go-hclog"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/atomic"
	"google.golang.org/grpc"
	anypb "google.golang.org/protobuf/types/known/anypb"
)

const (
	DefaultEpochSize = 100000
	// When threshold reached, we mark it as a really annoying contract
	_annoyingContractThrshold = 3

	WriteBlockSource = "ibft"
)

var (
	ErrInvalidHookParam      = errors.New("invalid IBFT hook param passed in")
	ErrInvalidMechanismType  = errors.New("invalid consensus mechanism type in params")
	ErrMissingMechanismType  = errors.New("missing consensus mechanism type in params")
	ErrEmptyValidatorExtract = errors.New("empty extract validatorset")
	ErrInvalidMixHash        = errors.New("invalid mixhash")
	ErrInvalidUncleHash      = errors.New("invalid uncle hash")
	ErrWrongDifficulty       = errors.New("wrong difficulty")
	ErrInvalidBlockTimestamp = errors.New("invalid block timestamp")
	ErrInvalidCommittedSeal  = errors.New("invalid committed seal")
)

type blockchainInterface interface {
	Header() *types.Header
	GetHeaderByNumber(i uint64) (*types.Header, bool)
	WriteBlock(block *types.Block, source string) error
	VerifyPotentialBlock(block *types.Block) error
	CalculateGasLimit(number uint64) (uint64, error)
	SubscribeEvents() blockchain.Subscription
}

type ddosProtectionInterface interface {
	IsDDOSTx(tx *types.Transaction) bool
	MarkDDOSTx(tx *types.Transaction)
}

type txPoolInterface interface {
	ddosProtectionInterface
	Drop(tx *types.Transaction)
	DemoteAllPromoted(tx *types.Transaction, correctNonce uint64)
	ResetWithHeaders(headers ...*types.Header)
	Pending() map[types.Address][]*types.Transaction
}

// Ibft represents the IBFT consensus mechanism object
type Ibft struct {
	sealing bool // Flag indicating if the node is a sealer

	logger hclog.Logger               // Output logger
	config *consensus.Config          // Consensus configuration
	Grpc   *grpc.Server               // gRPC configuration
	state  *currentstate.CurrentState // Reference to the current state

	blockchain blockchainInterface // Interface exposed by the blockchain layer
	executor   *state.Executor     // Reference to the state executor
	closeCh    chan struct{}       // Channel for closing
	isClosed   *atomic.Bool

	validatorKey     *ecdsa.PrivateKey // Private key for the validator
	validatorKeyAddr types.Address

	txpool txPoolInterface // Reference to the transaction pool

	store     *snapshotStore // Snapshot store that keeps track of all snapshots
	epochSize uint64

	msgQueue *msgQueue     // Structure containing different message queues
	updateCh chan struct{} // Update channel

	syncer protocol.Syncer // Reference to the sync protocol

	network   network.Server // Reference to the networking layer
	transport transport      // Reference to the transport protocol

	operator *operator

	// aux test methods
	forceTimeoutCh bool

	metrics *consensus.Metrics

	secretsManager secrets.SecretsManager

	mechanisms []ConsensusMechanism // IBFT ConsensusMechanism used (PoA / PoS)

	blockTime time.Duration // Minimum block generation time in seconds

	currentValidators    validator.Validators // Validator set at current sequence
	currentValidatorsMux sync.RWMutex         // Mutex for currentValidators
	// Recording resource exhausting contracts
	// but would not banish it until it became a real ddos attack
	// not thread safe, but can be used sequentially
	exhaustingContracts map[types.Address]uint64

	// consensus associated
	cancelSequence context.CancelFunc
	wg             sync.WaitGroup
}

// runHook runs a specified hook if it is present in the hook map
func (i *Ibft) runHook(hookName HookType, height uint64, hookParam interface{}) error {
	for _, mechanism := range i.mechanisms {
		if !mechanism.IsAvailable(hookName, height) {
			continue
		}

		// Grab the hook map
		hookMap := mechanism.GetHookMap()

		// Grab the actual hook if it's present
		hook, ok := hookMap[hookName]
		if !ok {
			// hook not found, continue
			continue
		}

		// Run the hook
		if err := hook(hookParam); err != nil {
			return fmt.Errorf("error occurred during a call of %s hook in %s: %w", hookName, mechanism.GetType(), err)
		}
	}

	return nil
}

// Factory implements the base consensus Factory method
func Factory(
	params *consensus.ConsensusParams,
) (consensus.Consensus, error) {
	var epochSize uint64
	if definedEpochSize, ok := params.Config.Config[KeyEpochSize]; !ok {
		// No epoch size defined, use the default one
		epochSize = DefaultEpochSize
	} else {
		// Epoch size is defined, use the passed in one
		readSize, ok := definedEpochSize.(float64)
		if !ok {
			return nil, errors.New("epochSize invalid type assertion")
		}

		epochSize = uint64(readSize)

		if epochSize == 0 {
			// epoch size should never be zero.
			epochSize = DefaultEpochSize
		}
	}

	p := &Ibft{
		logger:              params.Logger.Named("ibft"),
		config:              params.Config,
		Grpc:                params.Grpc,
		blockchain:          params.Blockchain,
		executor:            params.Executor,
		closeCh:             make(chan struct{}),
		isClosed:            atomic.NewBool(false),
		txpool:              params.Txpool,
		state:               &currentstate.CurrentState{},
		network:             params.Network,
		epochSize:           epochSize,
		sealing:             params.Seal,
		metrics:             params.Metrics,
		secretsManager:      params.SecretsManager,
		blockTime:           time.Duration(params.BlockTime) * time.Second,
		exhaustingContracts: make(map[types.Address]uint64),
	}

	// set up additional timeout for building block
	p.state.SetAdditionalTimeout(p.blockTime)

	// Initialize the mechanism
	if err := p.setupMechanism(); err != nil {
		return nil, err
	}

	// Istanbul requires a different header hash function
	types.HeaderHash = istanbulHeaderHash

	p.syncer = protocol.NewSyncer(
		params.Logger,
		params.Network,
		params.Blockchain,
		params.BlockBroadcast,
	)

	return p, nil
}

// Start starts the IBFT consensus
func (i *Ibft) Initialize() error {
	// register the grpc operator
	if i.Grpc != nil {
		i.operator = &operator{ibft: i}
		proto.RegisterIbftOperatorServer(i.Grpc, i.operator)
	}

	// Set up the node's validator key
	if err := i.createKey(); err != nil {
		return err
	}

	i.logger.Info("validator key", "addr", i.validatorKeyAddr.String())

	// start the transport protocol
	if err := i.setupTransport(); err != nil {
		return err
	}

	// // initialize fork manager
	// if err := i.forkManager.Initialize(); err != nil {
	// 	return err
	// }

	// Set up the snapshots
	if err := i.setupSnapshot(); err != nil {
		return err
	}

	// set up current module cache
	if err := i.updateCurrentModules(i.blockchain.Header().Number + 1); err != nil {
		return err
	}

	return nil
}

// Start starts the IBFT consensus
func (i *Ibft) Start() error {
	// Start the syncer
	if err := i.syncer.Start(); err != nil {
		return err
	}

	// Start syncing blocks from other peers
	go i.startSyncing()

	// Start the actual IBFT protocol
	go i.startConsensus()

	return nil
}

// sync runs the syncer in the background to receive blocks from advanced peers
func (i *Ibft) startSyncing() {
	logger := i.logger.Named("syncing")

	callInsertBlockHook := func(block *types.Block) bool {
		blockNumber := block.Number()

		// insert block
		if hookErr := i.runHook(InsertBlockHook, blockNumber, blockNumber); hookErr != nil {
			logger.Error("unable to run hook", "hook", InsertBlockHook, "err", hookErr)
		}

		// update module cache
		if err := i.updateCurrentModules(blockNumber + 1); err != nil {
			logger.Error("failed to update sub modules", "height", blockNumber+1, "err", err)
		}

		// reset headers of txpool
		i.txpool.ResetWithHeaders(block.Header)

		return false
	}

	if err := i.syncer.Sync(
		callInsertBlockHook,
	); err != nil {
		logger.Error("watch sync failed", "err", err)
	}
}

// GetSyncProgression gets the latest sync progression, if any
func (i *Ibft) GetSyncProgression() *progress.Progression {
	return i.syncer.GetSyncProgression()
}

type transport interface {
	Gossip(msg *proto.MessageReq) error
	Close() error
}

// Define the IBFT libp2p protocol
var ibftProto = "/ibft/0.1"

type gossipTransport struct {
	topic network.Topic
}

// Gossip publishes a new message to the topic
func (g *gossipTransport) Gossip(msg *proto.MessageReq) error {
	return g.topic.Publish(msg)
}

func (g *gossipTransport) Close() error {
	return g.topic.Close()
}

// GetIBFTForks returns IBFT fork configurations from chain config
func GetIBFTForks(ibftConfig map[string]interface{}) ([]IBFTFork, error) {
	// no fork, only specifying IBFT type in chain config
	if originalType, ok := ibftConfig["type"].(string); ok {
		typ, err := ParseType(originalType)
		if err != nil {
			return nil, err
		}

		return []IBFTFork{
			{
				Type:       typ,
				Deployment: nil,
				From:       common.JSONNumber{Value: 0},
				To:         nil,
			},
		}, nil
	}

	// with forks
	if types, ok := ibftConfig["types"].([]interface{}); ok {
		bytes, err := json.Marshal(types)
		if err != nil {
			return nil, err
		}

		var forks []IBFTFork
		if err := json.Unmarshal(bytes, &forks); err != nil {
			return nil, err
		}

		return forks, nil
	}

	return nil, errors.New("current IBFT type not found")
}

// setupMechanism read current mechanism in params and sets up consensus mechanism
func (i *Ibft) setupMechanism() error {
	ibftForks, err := GetIBFTForks(i.config.Config)
	if err != nil {
		return err
	}

	i.mechanisms = make([]ConsensusMechanism, len(ibftForks))

	for idx, fork := range ibftForks {
		factory, ok := mechanismBackends[fork.Type]
		if !ok {
			return fmt.Errorf("consensus mechanism doesn't define: %s", fork.Type)
		}

		fork := fork
		if i.mechanisms[idx], err = factory(i, &fork); err != nil {
			return err
		}
	}

	return nil
}

// setupTransport sets up the gossip transport protocol
func (i *Ibft) setupTransport() error {
	// Define a new topic
	topic, err := i.network.NewTopic(ibftProto, &proto.MessageReq{})
	if err != nil {
		return err
	}

	// Subscribe to the newly created topic
	err = topic.Subscribe(func(obj interface{}, _ peer.ID) {
		if !i.isActiveValidator(i.validatorKeyAddr) {
			// we're not active validator, don't ever care about any ibft messages
			return
		}

		msg, ok := obj.(*proto.MessageReq)
		if !ok {
			i.logger.Error("invalid type assertion for message request")

			return
		}

		// decode sender
		if err := validateMsg(msg); err != nil {
			i.logger.Error("failed to validate msg", "err", err)

			return
		}

		if !i.isActiveValidator(msg.FromAddr()) {
			// TODO: punish bad node
			// ignore message from non-validator
			return
		}

		if msg.From == i.validatorKeyAddr.String() {
			// we are the sender, skip this message since we already
			// relay our own messages internally.
			return
		}

		i.pushMessage(msg)
	})

	if err != nil {
		return err
	}

	i.transport = &gossipTransport{topic: topic}

	return nil
}

// createKey sets the validator's private key from the secrets manager
func (i *Ibft) createKey() error {
	i.msgQueue = newMsgQueue()
	i.closeCh = make(chan struct{})
	i.updateCh = make(chan struct{})

	if i.validatorKey == nil {
		// Check if the validator key is initialized
		var key *ecdsa.PrivateKey

		if i.secretsManager.HasSecret(secrets.ValidatorKey) {
			// The validator key is present in the secrets manager, load it
			validatorKey, readErr := crypto.ReadConsensusKey(i.secretsManager)
			if readErr != nil {
				return fmt.Errorf("unable to read validator key from Secrets Manager, %w", readErr)
			}

			key = validatorKey
		} else {
			// The validator key is not present in the secrets manager, generate it
			validatorKey, validatorKeyEncoded, genErr := crypto.GenerateAndEncodePrivateKey()
			if genErr != nil {
				return fmt.Errorf("unable to generate validator key for Secrets Manager, %w", genErr)
			}

			// Save the key to the secrets manager
			saveErr := i.secretsManager.SetSecret(secrets.ValidatorKey, validatorKeyEncoded)
			if saveErr != nil {
				return fmt.Errorf("unable to save validator key to Secrets Manager, %w", saveErr)
			}

			key = validatorKey
		}

		i.validatorKey = key
		i.validatorKeyAddr = crypto.PubKeyToAddress(&key.PublicKey)
	}

	return nil
}

const IbftKeyName = "validator.key"

// startConsensus starts the IBFT consensus state machine
func (i *Ibft) startConsensus() {
	var (
		newBlockSub   = i.blockchain.SubscribeEvents()
		syncerBlockCh = make(chan struct{})
	)

	defer newBlockSub.Unsubscribe()

	// Receive a notification every time syncer manages
	// to insert a valid block. Used for cancelling active consensus
	// rounds for a specific height
	go func() {
		for {
			if newBlockSub.IsClosed() {
				return
			}

			ev, ok := <-newBlockSub.GetEvent()
			if ev == nil || !ok {
				i.logger.Debug("received nil event from blockchain subscription (ignoring)")

				continue
			}

			if ev.Source == protocol.WriteBlockSource {
				if ev.NewChain[0].Number < i.blockchain.Header().Number {
					// The blockchain notification system can eventually deliver
					// stale block notifications. These should be ignored
					continue
				}

				syncerBlockCh <- struct{}{}
			}
		}
	}()

	var (
		sequenceCh  = make(<-chan struct{})
		isValidator bool
	)

	for {
		var (
			latest  = i.blockchain.Header().Number
			pending = latest + 1
		)

		if err := i.updateCurrentModules(pending); err != nil {
			i.logger.Error(
				"failed to update submodules",
				"height", pending,
				"err", err,
			)
		}

		isValidator = i.isValidSnapshot()

		if isValidator {
			// i.startNewSequence()
			sequenceCh = i.runSequence(pending)
		}

		select {
		case <-syncerBlockCh:
			if isValidator {
				i.stopSequence()
				i.logger.Info("canceled sequence", "sequence", pending)
			}
		case <-sequenceCh:
		case <-i.closeCh:
			if isValidator {
				i.stopSequence()
				i.logger.Info("ibft close", "sequence", pending)
			}

			return
		}
	}
}

// isValidSnapshot checks if the current node is in the validator set for the latest snapshot
func (i *Ibft) isValidSnapshot() bool {
	if !i.isSealing() {
		return false
	}

	// check if we are a validator and enabled
	header := i.blockchain.Header()
	snap, err := i.getSnapshot(header.Number)

	if err != nil {
		return false
	}

	if snap.Set.Includes(i.validatorKeyAddr) {
		return true
	}

	return false
}

// shouldWriteSystemTransactions checks whether system contract transaction should write at given height
//
// only active after detroit hardfork
func (i *Ibft) shouldWriteSystemTransactions(height uint64) bool {
	if i.config == nil || i.config.Params == nil || i.config.Params.Forks == nil { // old logic test
		return false
	}

	return i.config.Params.Forks.At(height).Detroit
}

// shouldWriteTransactions checks if each consensus mechanism accepts a block with transactions at given height
// returns true if all mechanisms accept
// otherwise return false
func (i *Ibft) shouldWriteTransactions(height uint64) bool {
	for _, m := range i.mechanisms {
		if m.ShouldWriteTransactions(height) {
			return true
		}
	}

	return false
}

// buildBlock builds the block, based on the passed in snapshot and parent header
func (i *Ibft) buildBlock(snap *Snapshot, parent *types.Header) (*types.Block, error) {
	header := &types.Header{
		ParentHash: parent.Hash,
		Number:     parent.Number + 1,
		Miner:      i.validatorKeyAddr,
		Nonce:      types.Nonce{},
		MixHash:    IstanbulDigest,
		// this is required because blockchain needs difficulty to organize blocks and forks
		Difficulty: parent.Number + 1,
		StateRoot:  types.EmptyRootHash, // this avoids needing state for now
		Sha3Uncles: types.EmptyUncleHash,
		GasLimit:   parent.GasLimit, // Inherit from parent for now, will need to adjust dynamically later.
	}

	// calculate gas limit based on parent header
	gasLimit, err := i.blockchain.CalculateGasLimit(header.Number)
	if err != nil {
		return nil, err
	}

	// update gas limit first
	header.GasLimit = gasLimit

	if hookErr := i.runHook(CandidateVoteHook, header.Number, &candidateVoteHookParams{
		header: header,
		snap:   snap,
	}); hookErr != nil {
		i.logger.Error("unable to run hook when building block", "hook", CandidateVoteHook, "err", hookErr)
	}

	// set the brocasting timestamp if possible
	// must use parent timestamp,
	parentTime := time.Unix(int64(parent.Timestamp), 0)
	headerTime := parentTime.Add(i.blockTime)
	now := time.Now()

	if headerTime.Before(now) {
		headerTime = now
	}

	header.Timestamp = uint64(headerTime.Unix())

	// we need to include in the extra field the current set of validators
	putIbftExtraValidators(header, snap.Set)

	transition, err := i.executor.BeginTxn(parent.StateRoot, header, i.validatorKeyAddr)
	if err != nil {
		return nil, err
	}

	// upgrade system if needed
	upgrader.UpgradeSystem(
		i.config.Params.ChainID,
		i.config.Params.Forks,
		header.Number,
		transition.Txn(),
		i.logger,
	)

	// If the mechanism is PoS -> build a regular block if it's not an end-of-epoch block
	// If the mechanism is PoA -> always build a regular block, regardless of epoch
	var (
		txs         []*types.Transaction
		includedTxs []*types.Transaction
		dropTxs     []*types.Transaction
		resetTxs    []*demoteTransaction
	)

	// insert normal transactions
	if i.shouldWriteTransactions(header.Number) {
		includedTxs, dropTxs, resetTxs = i.writeTransactions(gasLimit, transition, headerTime.Add(i.blockTime))
		txs = append(txs, includedTxs...)
	}

	// insert system transactions at last to ensure it works
	if i.shouldWriteSystemTransactions(header.Number) {
		systemTxs, err := i.writeSystemTxs(transition, parent, header)
		if err != nil {
			return nil, err
		}

		txs = append(txs, systemTxs...)
	}

	if err := i.PreStateCommit(header, transition); err != nil {
		return nil, err
	}

	_, root, err := transition.Commit()
	if err != nil {
		return nil, err
	}

	header.StateRoot = root
	header.GasUsed = transition.TotalGas()

	// build the block
	block := consensus.BuildBlock(consensus.BuildBlockParams{
		Header:   header,
		Txns:     txs,
		Receipts: transition.Receipts(),
	})

	// write the seal of the block after all the fields are completed
	header, err = writeSeal(i.validatorKey, block.Header)
	if err != nil {
		return nil, err
	}

	block.Header = header

	// compute the hash, this is only a provisional hash since the final one
	// is sealed after all the committed seals
	block.Header.ComputeHash()

	// TODO: remove these logic. ibft should not manipulate the txpool status.
	// drop account txs first
	for _, tx := range dropTxs {
		i.txpool.Drop(tx)
	}
	// demote account txs
	for _, tx := range resetTxs {
		i.txpool.DemoteAllPromoted(tx.Tx, tx.CorrectNonce)
	}

	i.logger.Info("build block",
		"number", header.Number,
		"txs", len(txs),
		"dropTxs", len(dropTxs),
		"resetTxs", len(resetTxs),
	)

	return block, nil
}

func (i *Ibft) writeSystemSlashTx(
	transition *state.Transition,
	parent, header *types.Header,
) (*types.Transaction, error) {
	if i.currentRound() == 0 {
		// no need slashing
		return nil, nil
	}

	// only punish the first validator
	lastBlockProposer, _ := ecrecoverFromHeader(parent)

	needPunished := i.state.CalcNeedPunished(i.currentRound(), lastBlockProposer)
	if len(needPunished) == 0 {
		// it shouldn't be, but we still need to prevent overwhelming
		return nil, nil
	}

	tx, err := i.makeTransitionSlashTx(transition.Txn(), header.Number, needPunished[0])
	if err != nil {
		return nil, err
	}

	// system transaction, increase gas limit if needed
	increaseHeaderGasIfNeeded(transition, header, tx)

	// execute slash tx
	if err := transition.Write(tx); err != nil {
		return nil, err
	}

	return tx, nil
}

func (i *Ibft) writeSystemDepositTx(
	transition *state.Transition,
	header *types.Header,
) (*types.Transaction, error) {
	// make deposit tx
	tx, err := i.makeTransitionDepositTx(transition.Txn(), header.Number)
	if err != nil {
		return nil, err
	}

	// system transaction, increase gas limit if needed
	increaseHeaderGasIfNeeded(transition, header, tx)

	// execute deposit tx
	if err := transition.Write(tx); err != nil {
		return nil, err
	}

	return tx, nil
}

func (i *Ibft) writeSystemTxs(
	transition *state.Transition,
	parent, header *types.Header,
) (txs []*types.Transaction, err error) {
	// slash transaction
	slashTx, err := i.writeSystemSlashTx(transition, parent, header)
	if err != nil {
		return nil, err
	} else if slashTx != nil {
		txs = append(txs, slashTx)
	}

	// deposit transaction
	depositTx, err := i.writeSystemDepositTx(transition, header)
	if err != nil {
		return nil, err
	}

	txs = append(txs, depositTx)

	return txs, nil
}

func increaseHeaderGasIfNeeded(transition *state.Transition, header *types.Header, tx *types.Transaction) {
	if transition.TotalGas()+tx.Gas <= header.GasLimit {
		return
	}

	extractAmount := transition.TotalGas() + tx.Gas - header.GasLimit

	// increase it
	header.GasLimit += extractAmount
	transition.IncreaseSystemTransactionGas(extractAmount)
}

func (i *Ibft) currentRound() uint64 {
	return i.state.Round()
}

func (i *Ibft) makeTransitionDepositTx(
	txn *state.Txn,
	height uint64,
) (*types.Transaction, error) {
	// singer
	signer := i.getSigner(height)

	// make deposit tx
	tx, err := validatorset.MakeDepositTx(txn, i.validatorKeyAddr)
	if err != nil {
		return nil, err
	}

	// sign tx
	tx, err = signer.SignTx(tx, i.validatorKey)
	if err != nil {
		return nil, err
	}

	return tx, err
}

func (i *Ibft) makeTransitionSlashTx(
	txn *state.Txn,
	height uint64,
	needPunished types.Address,
) (*types.Transaction, error) {
	// singer
	signer := i.getSigner(height)

	// make deposit tx
	tx, err := validatorset.MakeSlashTx(txn, i.validatorKeyAddr, needPunished)
	if err != nil {
		return nil, err
	}

	// sign tx
	tx, err = signer.SignTx(tx, i.validatorKey)
	if err != nil {
		return nil, err
	}

	return tx, err
}

func (i *Ibft) isActiveValidator(addr types.Address) bool {
	i.currentValidatorsMux.RLock()
	defer i.currentValidatorsMux.RUnlock()

	return i.currentValidators.Includes(addr)
}

// updateCurrentModules updates Txsigner and Validators
// that are used at specified height
func (i *Ibft) updateCurrentModules(height uint64) error {
	snap, err := i.getSnapshot(height)
	if err != nil {
		return err
	}

	i.currentValidatorsMux.Lock()
	defer i.currentValidatorsMux.Unlock()

	i.currentValidators = snap.Set

	i.logger.Info("update current module",
		"height", height,
		"validators", i.currentValidators,
	)

	return nil
}

func (i *Ibft) getSigner(height uint64) crypto.TxSigner {
	return crypto.NewSigner(
		i.config.Params.Forks.At(height),
		uint64(i.config.Params.ChainID),
	)
}

type transitionInterface interface {
	Write(txn *types.Transaction) error
	WriteFailedReceipt(txn *types.Transaction) error
}

type demoteTransaction struct {
	Tx           *types.Transaction
	CorrectNonce uint64
}

// writeTransactions writes transactions from the txpool to the transition object
// and returns transactions that were included in the transition (new block)
func (i *Ibft) writeTransactions(
	gasLimit uint64,
	transition transitionInterface,
	terminalTime time.Time,
) (
	includedTransactions []*types.Transaction,
	shouldDropTxs []*types.Transaction,
	shouldDemoteTxs []*demoteTransaction,
) {
	// get all pending transactions once and for all
	pendingTxs := i.txpool.Pending()
	// get highest price transaction queue
	priceTxs := types.NewTransactionsByPriceAndNonce(pendingTxs)

	for {
		// terminate transaction executing once timeout
		if i.shouldTerminate(terminalTime) {
			i.logger.Info("block building time exceeds")

			break
		}

		tx := priceTxs.Peek()
		if tx == nil {
			i.logger.Info("no more transactions")

			break
		}

		if i.shouldMarkLongConsumingTx(tx) {
			// count attack
			i.countDDOSAttack(tx)
		}

		if i.txpool.IsDDOSTx(tx) {
			i.logger.Info("drop ddos attack contract transaction",
				"address", tx.To,
				"from", tx.From,
			)
			// don't forget to pop the transaction if not execute it
			priceTxs.Pop()

			// drop tx
			shouldDropTxs = append(shouldDropTxs, tx)

			continue
		}

		if tx.ExceedsBlockGasLimit(gasLimit) {
			// the account transactions should be dropped
			shouldDropTxs = append(shouldDropTxs, tx)
			// The address is punished. For current loop, it would not include its transactions any more.
			priceTxs.Pop()
			// write failed receipts
			if err := transition.WriteFailedReceipt(tx); err != nil {
				i.logger.Error("write receipt failed", "err", err)
			}

			continue
		}

		begin := time.Now() // for duration calculation

		if err := transition.Write(tx); err != nil {
			// mark long time consuming contract to prevent ddos attack
			i.markLongTimeConsumingContract(tx, begin)

			i.logger.Debug("write transaction failed", "hash", tx.Hash, "from", tx.From,
				"nonce", tx.Nonce, "err", err)

			//nolint:errorlint
			if _, ok := err.(*state.AllGasUsedError); ok {
				// no more transaction could be packed
				i.logger.Debug("Not enough gas for further transactions")

				break
			} else if _, ok := err.(*state.GasLimitReachedTransitionApplicationError); ok {
				// Ignore transaction when the free gas not enough
				i.logger.Debug("Gas limit exceeded for current block", "from", tx.From)
				priceTxs.Pop()
			} else if nonceErr, ok := err.(*state.NonceTooLowError); ok {
				// low nonce tx, should reset accounts once done
				i.logger.Warn("write transaction nonce too low",
					"hash", tx.Hash, "from", tx.From, "nonce", tx.Nonce)
				// skip the address, whose txs should be reset first.
				shouldDemoteTxs = append(shouldDemoteTxs, &demoteTransaction{tx, nonceErr.CorrectNonce})
				// priceTxs.Shift()
				priceTxs.Pop()
			} else if nonceErr, ok := err.(*state.NonceTooHighError); ok {
				// high nonce tx, should reset accounts once done
				i.logger.Error("write miss some transactions with higher nonce",
					tx.Hash, "from", tx.From, "nonce", tx.Nonce)
				shouldDemoteTxs = append(shouldDemoteTxs, &demoteTransaction{tx, nonceErr.CorrectNonce})
				priceTxs.Pop()
			} else {
				// no matter what kind of failure, drop is reasonable for not executed it yet
				i.logger.Debug("write not executed transaction failed",
					"hash", tx.Hash, "from", tx.From,
					"nonce", tx.Nonce, "err", err)
				shouldDropTxs = append(shouldDropTxs, tx)
				priceTxs.Pop()
			}

			continue
		}

		// no errors, go on
		priceTxs.Shift()
		// mark long time consuming contract to prevent ddos attack
		i.markLongTimeConsumingContract(tx, begin)

		includedTransactions = append(includedTransactions, tx)
	}

	i.logger.Info("executed txns",
		"successful", len(includedTransactions),
		"shouldDropTxs", len(shouldDropTxs),
		"shouldDemoteTxs", len(shouldDemoteTxs),
	)

	return
}

func (i *Ibft) shouldTerminate(terminalTime time.Time) bool {
	return time.Now().After(terminalTime)
}

func (i *Ibft) shouldMarkLongConsumingTx(tx *types.Transaction) bool {
	if tx.To == nil {
		return false
	}

	count, exists := i.exhaustingContracts[*tx.To]

	return exists && count >= _annoyingContractThrshold
}

func (i *Ibft) countDDOSAttack(tx *types.Transaction) {
	i.txpool.MarkDDOSTx(tx)
}

func (i *Ibft) markLongTimeConsumingContract(tx *types.Transaction, begin time.Time) {
	duration := time.Since(begin).Milliseconds()
	// long contract creation is tolerable, long time execution is not tolerable
	if tx.To == nil || duration < i.blockTime.Milliseconds() {
		return
	}

	// banish the contract
	count := i.exhaustingContracts[*tx.To]
	count++
	i.exhaustingContracts[*tx.To] = count

	i.logger.Info("mark contract who consumes too many CPU or I/O time",
		"duration", duration,
		"count", count,
		"from", tx.From,
		"to", tx.To,
		"gasPrice", tx.GasPrice,
		"gas", tx.Gas,
		"hash", tx.Hash(),
	)
}

func (i *Ibft) isDepositTx(height uint64, coinbase types.Address, tx *types.Transaction) bool {
	if tx.To == nil || *tx.To != systemcontracts.AddrValidatorSetContract {
		return false
	}

	// check input
	if !validatorset.IsDepositTransactionSignture(tx.Input) {
		return false
	}

	// signer by height
	signer := i.getSigner(height)

	// tx sender
	from, err := signer.Sender(tx)
	if err != nil {
		return false
	}

	return from == coinbase
}

func (i *Ibft) isSlashTx(height uint64, coinbase types.Address, tx *types.Transaction) bool {
	if tx.To == nil || *tx.To != systemcontracts.AddrValidatorSetContract {
		return false
	}

	// check input
	if !validatorset.IsSlashTransactionSignture(tx.Input) {
		return false
	}

	// signer by height
	signer := i.getSigner(height)

	// tx sender
	from, err := signer.Sender(tx)
	if err != nil {
		return false
	}

	return from == coinbase
}

// updateMetrics will update various metrics based on the given block
// currently we capture No.of Txs and block interval metrics using this function
func (i *Ibft) updateMetrics(block *types.Block) {
	// get previous header
	prvHeader, _ := i.blockchain.GetHeaderByNumber(block.Number() - 1)
	parentTime := time.Unix(int64(prvHeader.Timestamp), 0)
	headerTime := time.Unix(int64(block.Header.Timestamp), 0)

	//Update the block interval metric
	if block.Number() > 1 {
		i.metrics.SetBlockInterval(
			headerTime.Sub(parentTime).Seconds(),
		)
	}

	//Update the Number of transactions in the block metric
	i.metrics.SetNumTxs(float64(len(block.Body().Transactions)))
}

func gatherCanonicalCommittedSeals(
	canonicalSeals []string,
	validators validator.Validators,
	block *types.Block,
) ([][]byte, error) {
	committedSeals := make([][]byte, 0, len(canonicalSeals))

	for _, commit := range canonicalSeals {
		// no need to check the format of seal here because writeCommittedSeals will check
		seal, addr, err := committedSealFromHex(commit, block.Hash())
		if err != nil {
			return nil, err
		}

		// check whether seals from validators
		if !validators.Includes(addr) {
			return nil, ErrInvalidCommittedSeal
		}

		committedSeals = append(committedSeals, seal)
	}

	return committedSeals, nil
}

func (i *Ibft) insertBlock(block *types.Block) error {
	// Gather the committed seals for the block
	committedSeals, err := gatherCanonicalCommittedSeals(
		i.state.CanonicalSeal().Canonical.Seals,
		i.currentValidators,
		block,
	)
	if err != nil {
		return err
	}

	// Push the committed seals to the header
	header, err := writeCommittedSeals(block.Header, committedSeals)
	if err != nil {
		return err
	}

	// The hash needs to be recomputed since the extra data was changed
	block.Header = header
	block.Header.ComputeHash()

	// Verify the header only, since the block body is already verified
	if err := i.VerifyHeader(block.Header); err != nil {
		return err
	}

	// Save the block locally
	if err := i.blockchain.WriteBlock(block, WriteBlockSource); err != nil {
		return err
	}

	if hookErr := i.runHook(InsertBlockHook, header.Number, header.Number); hookErr != nil {
		return hookErr
	}

	i.logger.Info(
		"block committed",
		"sequence", i.state.Sequence(),
		"hash", block.Hash(),
		"validators", len(i.state.Validators()),
		"rounds", i.state.Round()+1,
		"committed", i.state.NumCommitted(),
	)

	// after the block has been written we reset the txpool so that
	// the old transactions are removed
	i.txpool.ResetWithHeaders(block.Header)

	return nil
}

var (
	errIncorrectBlockLocked    = errors.New("block locked is incorrect")
	errIncorrectBlockHeight    = errors.New("proposed block number is incorrect")
	errBlockVerificationFailed = errors.New("block verification failed")
	errFailedToInsertBlock     = errors.New("failed to insert block")
)

func (i *Ibft) handleStateErr(err error) {
	i.state.HandleErr(err)
	i.setState(currentstate.RoundChangeState)
}

// --- com wrappers ---

func (i *Ibft) sendRoundChange() {
	i.gossip(proto.MessageReq_RoundChange)
}

func (i *Ibft) sendPreprepareMsg() {
	i.gossip(proto.MessageReq_Preprepare)
}

func (i *Ibft) sendPrepareMsg() {
	i.gossip(proto.MessageReq_Prepare)
}

func (i *Ibft) sendCommitMsg() {
	i.gossip(proto.MessageReq_Commit)
}

func (i *Ibft) sendPostCommitMsg() {
	i.gossip(proto.MessageReq_PostCommit)
}

func (i *Ibft) gossip(typ proto.MessageReq_Type) {
	msg := &proto.MessageReq{
		Type: typ,
		View: i.state.View().Copy(), // add View, all state needs
	}

	// if we are sending a preprepare message we need to include the proposed block
	if msg.Type == proto.MessageReq_Preprepare {
		msg.Proposal = &anypb.Any{
			Value: i.state.Block().MarshalRLP(),
		}
	}

	// if the message is commit, we need to add the committed seal
	if msg.Type == proto.MessageReq_Commit {
		seal, err := writeCommittedSeal(i.validatorKey, i.state.Block().Header)
		if err != nil {
			i.logger.Error("failed to commit seal", "err", err)

			return
		}

		msg.Seal = hex.EncodeToHex(seal)
	}

	if msg.Type == proto.MessageReq_PostCommit {
		committeds := i.state.Committed()
		seals := make([]string, 0, len(committeds))

		for _, committed := range committeds {
			seals = append(seals, committed.Seal)
		}

		msg.Canonical = &proto.CanonicalSeal{
			Hash:  i.state.Block().Hash().String(),
			Seals: seals,
		}
	}

	if msg.Type != proto.MessageReq_Preprepare {
		// send a copy to ourselves so that we can process this message as well
		msg2 := msg.Copy()
		msg2.From = i.validatorKeyAddr.String()
		i.pushMessage(msg2)
	}

	if err := signMsg(i.validatorKey, msg); err != nil {
		i.logger.Error("failed to sign message", "err", err)

		return
	}

	if err := i.transport.Gossip(msg); err != nil {
		i.logger.Error("failed to gossip", "err", err)
	}
}

// forceTimeout sets the forceTimeoutCh flag to true
func (i *Ibft) forceTimeout() {
	i.forceTimeoutCh = true
}

// isSealing checks if the current node is sealing blocks
func (i *Ibft) isSealing() bool {
	return i.sealing
}

// verifyHeaderImpl implements the actual header verification logic
func (i *Ibft) verifyHeaderImpl(snap *Snapshot, parent, header *types.Header) error {
	// ensure the extra data is correctly formatted
	extract, err := getIbftExtra(header)
	if err != nil {
		return err
	}
	// ensure validatorset exists in extra data
	if len(extract.Validators) == 0 {
		return ErrEmptyValidatorExtract
	}

	if hookErr := i.runHook(VerifyHeadersHook, header.Number, header.Nonce); hookErr != nil {
		return hookErr
	}

	if header.MixHash != IstanbulDigest {
		return ErrInvalidMixHash
	}

	if header.Sha3Uncles != types.EmptyUncleHash {
		return ErrInvalidUncleHash
	}

	// difficulty has to match number
	if header.Difficulty != header.Number {
		return ErrWrongDifficulty
	}

	// check timestamp
	if i.shouldVerifyTimestamp(header.Number) {
		// The diff between block timestamp and 'now' should not exceeds timeout.
		// Timestamp ascending array [parentTs, blockTs, now+blockTimeout]
		before, after := parent.Timestamp, uint64(time.Now().Add(i.blockTime).Unix())

		// header timestamp should not goes back
		if header.Timestamp <= before || header.Timestamp > after {
			i.logger.Debug("future blocktime invalid",
				"before", before,
				"after", after,
				"current", header.Timestamp,
			)

			return ErrInvalidBlockTimestamp
		}
	}

	// verify the sealer
	if err := verifySigner(snap, header); err != nil {
		return err
	}

	return nil
}

// shouldVerifyTimeStamp checks whether block timestamp should be verified or not
//
// only active after detroit hardfork
func (i *Ibft) shouldVerifyTimestamp(height uint64) bool {
	// backward compatible
	if i.config == nil || i.config.Params == nil || i.config.Params.Forks == nil {
		return false
	}

	return i.config.Params.Forks.IsDetroit(height)
}

// VerifyHeader wrapper for verifying headers
func (i *Ibft) VerifyHeader(header *types.Header) error {
	parent, ok := i.blockchain.GetHeaderByNumber(header.Number - 1)
	if !ok {
		return fmt.Errorf(
			"unable to get parent header for block number %d",
			header.Number,
		)
	}

	snap, err := i.getSnapshot(parent.Number)
	if err != nil {
		return err
	}

	// verify all the header fields + seal
	if err := i.verifyHeaderImpl(snap, parent, header); err != nil {
		return err
	}

	// verify the committed seals
	if err := verifyCommittedFields(snap, header); err != nil {
		return err
	}

	return nil
}

func (i *Ibft) ProcessHeaders(headers []*types.Header) error {
	return i.processHeaders(headers)
}

// GetBlockCreator retrieves the block signer from the extra data field
func (i *Ibft) GetBlockCreator(header *types.Header) (types.Address, error) {
	return ecrecoverFromHeader(header)
}

// PreStateCommit a hook to be called before finalizing state transition on inserting block
func (i *Ibft) PreStateCommit(header *types.Header, txn *state.Transition) error {
	params := &preStateCommitHookParams{
		header: header,
		txn:    txn,
	}
	if hookErr := i.runHook(PreStateCommitHook, header.Number, params); hookErr != nil {
		return hookErr
	}

	return nil
}

func (i *Ibft) IsSystemTransaction(height uint64, coinbase types.Address, tx *types.Transaction) bool {
	// only active after detroit hardfork
	if !i.shouldWriteSystemTransactions(height) {
		return false
	}

	if i.isDepositTx(height, coinbase, tx) {
		return true
	}

	return i.isSlashTx(height, coinbase, tx)
}

// GetEpoch returns the current epoch
func (i *Ibft) GetEpoch(number uint64) uint64 {
	if number%i.epochSize == 0 {
		return number / i.epochSize
	}

	return number/i.epochSize + 1
}

// IsLastOfEpoch checks if the block number is the last of the epoch
func (i *Ibft) IsLastOfEpoch(number uint64) bool {
	return number > 0 && number%i.epochSize == 0
}

// Close closes the IBFT consensus mechanism, and does write back to disk
func (i *Ibft) Close() error {
	if !i.isClosed.CAS(false, true) {
		i.logger.Error("IBFT consensus is Closed")

		return nil
	}

	close(i.closeCh)

	if i.config.Path != "" {
		err := i.store.saveToPath(i.config.Path)

		if err != nil {
			return err
		}
	}

	i.transport.Close()

	return nil
}

// getNextMessage reads a new message from the message queue
//
// continuable indicates whether it can continue to obtain data from the message channel
func (i *Ibft) getNextMessage(ctx context.Context, timeout time.Duration) (req *proto.MessageReq, continuable bool) {
	timeoutCh := time.NewTimer(timeout)
	defer timeoutCh.Stop()

	for {
		msg := i.msgQueue.readMessage(i.getState(), i.state.View())
		if msg != nil {
			return msg.obj, true
		}

		if i.forceTimeoutCh {
			i.forceTimeoutCh = false

			return nil, true
		}

		// wait until there is a new message or
		// someone closes the stopCh (i.e. timeout for round change)
		select {
		case <-timeoutCh.C:
			i.logger.Info("unable to read new message from the message queue", "timeout expired", timeout)

			return nil, true
		case <-ctx.Done():
			return nil, false
		case <-i.closeCh:
			return nil, false
		case <-i.updateCh:
		}
	}
}

// pushMessage pushes a new message to the message queue
func (i *Ibft) pushMessage(msg *proto.MessageReq) {
	task := &msgTask{
		view: msg.View,
		msg:  protoTypeToMsg(msg.Type),
		obj:  msg,
	}
	i.msgQueue.pushMessage(task)

	select {
	case i.updateCh <- struct{}{}:
	default:
	}
}

// startNewSequence changes the sequence and resets the round in the view of state
func (i *Ibft) startNewSequence() {
	header := i.blockchain.Header()

	i.state.SetView(&proto.View{
		Sequence: header.Number + 1,
		Round:    0,
	})
}

// startNewRound changes the round in the view of state
func (i *Ibft) startNewRound(newRound uint64) {
	i.state.SetView(&proto.View{
		Sequence: i.state.Sequence(),
		Round:    newRound,
	})
}
