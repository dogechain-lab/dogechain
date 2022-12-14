package state

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"sync/atomic"
	"time"

	"github.com/dogechain-lab/dogechain/chain"
	"github.com/dogechain-lab/dogechain/contracts/bridge"
	"github.com/dogechain-lab/dogechain/contracts/systemcontracts"
	"github.com/dogechain-lab/dogechain/crypto"
	"github.com/dogechain-lab/dogechain/state/runtime"
	"github.com/dogechain-lab/dogechain/state/runtime/evm"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/hashicorp/go-hclog"
)

const (
	spuriousDragonMaxCodeSize = 24576

	TxGas                 uint64 = 21000 // Per transaction not creating a contract
	TxGasContractCreation uint64 = 53000 // Per transaction that creates a contract
)

var emptyCodeHashTwo = types.BytesToHash(crypto.Keccak256(nil))

// GetHashByNumber returns the hash function of a block number
type GetHashByNumber = func(i uint64) types.Hash

type GetHashByNumberHelper = func(*types.Header) GetHashByNumber

// Executor is the main entity
type Executor struct {
	logger   hclog.Logger
	config   *chain.Params
	runtimes []runtime.Runtime
	state    State
	GetHash  GetHashByNumberHelper
	stopped  uint32 // atomic flag for stopping

	PostHook func(txn *Transition)
}

// NewExecutor creates a new executor
func NewExecutor(config *chain.Params, s State, logger hclog.Logger) *Executor {
	return &Executor{
		logger:   logger,
		config:   config,
		runtimes: []runtime.Runtime{},
		state:    s,
	}
}

func (e *Executor) WriteGenesis(alloc map[types.Address]*chain.GenesisAccount) (types.Hash, error) {
	snap := e.state.NewSnapshot()
	txn := NewTxn(e.state, snap)

	for addr, account := range alloc {
		if account.Balance != nil {
			txn.AddBalance(addr, account.Balance)
		}

		if account.Nonce != 0 {
			txn.SetNonce(addr, account.Nonce)
		}

		if len(account.Code) != 0 {
			txn.SetCode(addr, account.Code)
		}

		for key, value := range account.Storage {
			txn.SetState(addr, key, value)
		}
	}

	objs := txn.Commit(false)

	_, root, err := snap.Commit(objs)
	if err != nil {
		return types.ZeroHash, nil
	}

	return types.BytesToHash(root), nil
}

// SetRuntime adds a runtime to the runtime set
func (e *Executor) SetRuntime(r runtime.Runtime) {
	e.runtimes = append(e.runtimes, r)
}

type BlockResult struct {
	Root     types.Hash
	Receipts []*types.Receipt
	TotalGas uint64
}

// ProcessBlock already does all the handling of the whole process
func (e *Executor) ProcessTransactions(
	txn *Transition,
	gasLimit uint64,
	transactions []*types.Transaction,
) (*Transition, error) {
	for _, tx := range transactions {
		if e.IsStopped() {
			// halt more elegantly
			return nil, ErrExecutionStop
		}

		if tx.ExceedsBlockGasLimit(gasLimit) {
			if err := txn.WriteFailedReceipt(tx); err != nil {
				return nil, err
			}

			continue
		}

		if err := txn.Write(tx); err != nil {
			return nil, err
		}
	}

	return txn, nil
}

// ProcessBlock already does all the handling of the whole process
func (e *Executor) ProcessBlock(
	parentRoot types.Hash,
	block *types.Block,
	blockCreator types.Address,
) (*Transition, error) {
	txn, err := e.BeginTxn(parentRoot, block.Header, blockCreator)
	if err != nil {
		return nil, err
	}

	for _, tx := range block.Transactions {
		if e.IsStopped() {
			// halt more elegantly
			return nil, ErrExecutionStop
		}

		if tx.ExceedsBlockGasLimit(block.Header.GasLimit) {
			if err := txn.WriteFailedReceipt(tx); err != nil {
				return nil, err
			}

			continue
		}

		if err := txn.Write(tx); err != nil {
			return nil, err
		}
	}

	return txn, nil
}

func (e *Executor) IsStopped() bool {
	return atomic.LoadUint32(&e.stopped) > 0
}

func (e *Executor) Stop() {
	atomic.StoreUint32(&e.stopped, 1)
}

// StateAt returns snapshot at given root
func (e *Executor) State() State {
	return e.state
}

// StateAt returns snapshot at given root
func (e *Executor) StateAt(root types.Hash) (Snapshot, error) {
	return e.state.NewSnapshotAt(root)
}

// GetForksInTime returns the active forks at the given block height
func (e *Executor) GetForksInTime(blockNumber uint64) chain.ForksInTime {
	return e.config.Forks.At(blockNumber)
}

// TODO: It knows too much knowledge of the blockchain. Should limit its knowledge range,
// beginning from parameters.
func (e *Executor) BeginTxn(
	parentRoot types.Hash,
	header *types.Header,
	coinbaseReceiver types.Address,
) (*Transition, error) {
	config := e.config.Forks.At(header.Number)

	auxSnap2, err := e.state.NewSnapshotAt(parentRoot)
	if err != nil {
		return nil, err
	}

	newTxn := NewTxn(e.state, auxSnap2)

	env2 := runtime.TxContext{
		Coinbase:   coinbaseReceiver,
		Timestamp:  int64(header.Timestamp),
		Number:     int64(header.Number),
		Difficulty: types.BytesToHash(new(big.Int).SetUint64(header.Difficulty).Bytes()),
		GasLimit:   int64(header.GasLimit),
		ChainID:    int64(e.config.ChainID),
	}

	txn := &Transition{
		logger:   e.logger,
		r:        e,
		ctx:      env2,
		txn:      newTxn,
		getHash:  e.GetHash(header),
		auxState: e.state,
		config:   config,
		gasPool:  uint64(env2.GasLimit),

		receipts: []*types.Receipt{},
		totalGas: 0,
		// set a dummy tracer to 'collect' tracing
		evmLogger: runtime.NewDummyLogger(),
	}

	return txn, nil
}

type Transition struct {
	logger hclog.Logger

	// dummy
	auxState State

	r       *Executor
	config  chain.ForksInTime
	txn     *Txn
	getHash GetHashByNumber
	ctx     runtime.TxContext
	gasPool uint64

	// result
	receipts     []*types.Receipt
	totalGas     uint64
	totalGasHook func() uint64 // for testing

	// evmLogger for debugging, set a dummy logger to 'collect' tracing,
	// then we wouldn't have to judge any tracing flag
	evmLogger runtime.EVMLogger
	needDebug bool
}

// SetEVMLogger sets a non nil tracer to it
func (t *Transition) SetEVMLogger(logger runtime.EVMLogger) {
	t.evmLogger = logger

	switch logger.(type) {
	case nil:
		t.needDebug = false
	case *runtime.DummyLogger:
		t.needDebug = false
	default:
		t.needDebug = true
	}
}

func (t *Transition) GetEVMLogger() runtime.EVMLogger {
	return t.evmLogger
}

// HookTotalGas uses hook to return total gas
//
// Use it for testing
func (t *Transition) HookTotalGas(fn func() uint64) {
	t.totalGasHook = fn
}

func (t *Transition) TotalGas() uint64 {
	if t.totalGasHook != nil {
		return t.totalGasHook()
	}

	return t.totalGas
}

func (t *Transition) Receipts() []*types.Receipt {
	return t.receipts
}

var emptyFrom = types.Address{}

func (t *Transition) WriteFailedReceipt(txn *types.Transaction) error {
	signer := crypto.NewSigner(t.config, uint64(t.r.config.ChainID))

	if txn.From == emptyFrom {
		// Decrypt the from address
		from, err := signer.Sender(txn)
		if err != nil {
			return NewTransitionApplicationError(err, false)
		}

		txn.From = from
	}

	receipt := &types.Receipt{
		CumulativeGasUsed: t.totalGas,
		TxHash:            txn.Hash(),
		Logs:              t.txn.Logs(),
	}

	receipt.LogsBloom = types.CreateBloom([]*types.Receipt{receipt})
	receipt.SetStatus(types.ReceiptFailed)
	t.receipts = append(t.receipts, receipt)

	if txn.To == nil {
		receipt.ContractAddress = crypto.CreateAddress(txn.From, txn.Nonce).Ptr()
	}

	return nil
}

// Write writes another transaction to the executor
func (t *Transition) Write(txn *types.Transaction) error {
	signer := crypto.NewSigner(t.config, uint64(t.r.config.ChainID))

	var err error
	if txn.From == emptyFrom {
		// Decrypt the from address
		txn.From, err = signer.Sender(txn)
		if err != nil {
			return NewTransitionApplicationError(err, false)
		}
	}

	// Make a local copy and apply the transaction
	msg := txn.Copy()

	result, e := t.Apply(msg)
	if e != nil {
		t.logger.Debug("failed to apply tx", "err", e)

		return e
	}

	t.totalGas += result.GasUsed

	logs := t.txn.Logs()

	var root []byte

	receipt := &types.Receipt{
		CumulativeGasUsed: t.totalGas,
		TxHash:            txn.Hash(),
		GasUsed:           result.GasUsed,
	}

	if t.config.Byzantium {
		// The suicided accounts are set as deleted for the next iteration
		t.txn.CleanDeleteObjects(true)

		if result.Failed() {
			receipt.SetStatus(types.ReceiptFailed)
		} else {
			receipt.SetStatus(types.ReceiptSuccess)
		}
	} else {
		objs := t.txn.Commit(t.config.EIP155)
		ss, aux, err := t.txn.snapshot.Commit(objs)
		if err != nil {
			return err
		}

		t.txn = NewTxn(t.auxState, ss)
		root = aux
		receipt.Root = types.BytesToHash(root)
	}

	// if the transaction created a contract, store the creation address in the receipt.
	if msg.To == nil {
		receipt.ContractAddress = crypto.CreateAddress(msg.From, txn.Nonce).Ptr()
	}

	// handle cross bridge logs from|to dogecoin blockchain
	if err := t.handleBridgeLogs(msg, logs); err != nil {
		return err
	}

	// Set the receipt logs and create a bloom for filtering
	receipt.Logs = logs
	receipt.LogsBloom = types.CreateBloom([]*types.Receipt{receipt})
	t.receipts = append(t.receipts, receipt)

	return nil
}

func (t *Transition) handleBridgeLogs(msg *types.Transaction, logs []*types.Log) error {
	// filter bridge contract logs
	if len(logs) == 0 ||
		msg.To == nil ||
		*msg.To != systemcontracts.AddrBridgeContract {
		return nil
	}

	for _, log := range logs {
		if len(log.Topics) == 0 {
			continue
		}

		switch log.Topics[0] {
		case bridge.BridgeDepositedEventID:
			parsedLog, err := bridge.ParseBridgeDepositedLog(log)
			if err != nil {
				return err
			}

			t.txn.AddBalance(parsedLog.Receiver, parsedLog.Amount)
		case bridge.BridgeWithdrawnEventID:
			parsedLog, err := bridge.ParseBridgeWithdrawnLog(log)
			if err != nil {
				return err
			}

			// the total one is the real amount of Withdrawn event
			realAmount := big.NewInt(0).Add(parsedLog.Amount, parsedLog.Fee)

			if err := t.txn.SubBalance(parsedLog.Contract, realAmount); err != nil {
				return err
			}

			// the fee goes to system Vault contract
			t.txn.AddBalance(systemcontracts.AddrVaultContract, parsedLog.Fee)
		case bridge.BridgeBurnedEventID:
			parsedLog, err := bridge.ParseBridgeBurnedLog(log)
			if err != nil {
				return err
			}

			// burn
			if err := t.txn.SubBalance(parsedLog.Sender, parsedLog.Amount); err != nil {
				return err
			}
		}
	}

	return nil
}

// Commit commits the final result
func (t *Transition) Commit() (Snapshot, types.Hash, error) {
	objs := t.txn.Commit(t.config.EIP155)

	s2, root, err := t.txn.snapshot.Commit(objs)
	if err != nil {
		return nil, types.ZeroHash, err
	}

	return s2, types.BytesToHash(root), nil
}

func (t *Transition) subGasPool(amount uint64) error {
	if t.gasPool < amount {
		return ErrBlockLimitReached
	}

	t.gasPool -= amount

	return nil
}

// IncreaseSystemTransactionGas updates gas pool so that system contract transactions can be sealed.
func (t *Transition) IncreaseSystemTransactionGas(amount uint64) {
	t.addGasPool(amount)
	// don't forget to increase current context
	t.ctx.GasLimit += int64(amount)
}

func (t *Transition) addGasPool(amount uint64) {
	t.gasPool += amount
}

func (t *Transition) SetTxn(txn *Txn) {
	t.txn = txn
}

func (t *Transition) Txn() *Txn {
	return t.txn
}

// Apply applies a new transaction
func (t *Transition) Apply(msg *types.Transaction) (*runtime.ExecutionResult, error) {
	s := t.txn.Snapshot() //nolint:ifshort
	result, err := t.apply(msg)

	if err != nil {
		t.txn.RevertToSnapshot(s)
	}

	if t.r.PostHook != nil {
		t.r.PostHook(t)
	}

	return result, err
}

// ContextPtr returns reference of context
// This method is called only by test
func (t *Transition) ContextPtr() *runtime.TxContext {
	return &t.ctx
}

func (t *Transition) subGasLimitPrice(msg *types.Transaction) error {
	// deduct the upfront max gas cost
	upfrontGasCost := new(big.Int).Set(msg.GasPrice)
	upfrontGasCost.Mul(upfrontGasCost, new(big.Int).SetUint64(msg.Gas))

	if err := t.txn.SubBalance(msg.From, upfrontGasCost); err != nil {
		if errors.Is(err, runtime.ErrNotEnoughFunds) {
			return ErrNotEnoughFundsForGas
		}

		return err
	}

	return nil
}

func (t *Transition) nonceCheck(msg *types.Transaction) error {
	nonce := t.txn.GetNonce(msg.From)

	if msg.Nonce < nonce {
		return NewNonceTooLowError(fmt.Errorf("%w, actual: %d, wanted: %d", ErrNonceIncorrect, msg.Nonce, nonce), nonce)
	} else if msg.Nonce > nonce {
		return NewNonceTooHighError(fmt.Errorf("%w, actual: %d, wanted: %d", ErrNonceIncorrect, msg.Nonce, nonce), nonce)
	}

	return nil
}

// errors that can originate in the consensus rules checks of the apply method below
// surfacing of these errors reject the transaction thus not including it in the block

var (
	ErrNonceIncorrect        = errors.New("incorrect nonce")
	ErrNotEnoughFundsForGas  = errors.New("not enough funds to cover gas costs")
	ErrBlockLimitReached     = errors.New("gas limit reached in the pool")
	ErrIntrinsicGasOverflow  = errors.New("overflow in intrinsic gas calculation")
	ErrNotEnoughIntrinsicGas = errors.New("not enough gas supplied for intrinsic gas costs")
	ErrNotEnoughFunds        = errors.New("not enough funds for transfer with given value")
	ErrAllGasUsed            = errors.New("all gas used")
	ErrExecutionStop         = errors.New("execution stop")
)

type TransitionApplicationError struct {
	Err           error
	IsRecoverable bool // Should the transaction be discarded, or put back in the queue.
}

func (e *TransitionApplicationError) Error() string {
	return e.Err.Error()
}

func NewTransitionApplicationError(err error, isRecoverable bool) *TransitionApplicationError {
	return &TransitionApplicationError{
		Err:           err,
		IsRecoverable: isRecoverable,
	}
}

type NonceTooLowError struct {
	TransitionApplicationError
	CorrectNonce uint64
}

func NewNonceTooLowError(err error, correctNonce uint64) *NonceTooLowError {
	return &NonceTooLowError{
		*NewTransitionApplicationError(err, false),
		correctNonce,
	}
}

type NonceTooHighError struct {
	TransitionApplicationError
	CorrectNonce uint64
}

func NewNonceTooHighError(err error, correctNonce uint64) *NonceTooHighError {
	return &NonceTooHighError{
		*NewTransitionApplicationError(err, false),
		correctNonce,
	}
}

type GasLimitReachedTransitionApplicationError struct {
	TransitionApplicationError
}

func NewGasLimitReachedTransitionApplicationError(err error) *GasLimitReachedTransitionApplicationError {
	return &GasLimitReachedTransitionApplicationError{
		*NewTransitionApplicationError(err, true),
	}
}

type AllGasUsedError struct {
	TransitionApplicationError
}

func NewAllGasUsedError(err error) *AllGasUsedError {
	return &AllGasUsedError{
		*NewTransitionApplicationError(err, true),
	}
}

func (t *Transition) apply(msg *types.Transaction) (*runtime.ExecutionResult, error) {
	// First check this message satisfies all consensus rules before
	// applying the message. The rules include these clauses
	//
	// 0. the basic amount of gas is required
	// 1. the nonce of the message caller is correct
	// 2. caller has enough balance to cover transaction fee(gaslimit * gasprice)
	// 3. the amount of gas required is available in the block
	// 4. there is no overflow when calculating intrinsic gas
	// 5. the purchased gas is enough to cover intrinsic usage
	// 6. caller has enough balance to cover asset transfer for **topmost** call
	txn := t.txn

	t.logger.Debug("try to apply transaction",
		"hash", msg.Hash(), "from", msg.From, "nonce", msg.Nonce, "price", msg.GasPrice.String(),
		"remainingGas", t.gasPool, "wantGas", msg.Gas)

	// 0. the basic amount of gas is required
	if t.gasPool < TxGas {
		return nil, NewAllGasUsedError(ErrAllGasUsed)
	}

	// 1. the nonce of the message caller is correct
	if err := t.nonceCheck(msg); err != nil {
		return nil, err // the error already formatted
	}

	// 2. caller has enough balance to cover transaction fee(gaslimit * gasprice)
	if err := t.subGasLimitPrice(msg); err != nil {
		// It is not recoverable. All the transactions after that should be dropped
		return nil, NewTransitionApplicationError(err, true)
	}

	// 3. the amount of gas required is available in the block
	if err := t.subGasPool(msg.Gas); err != nil {
		return nil, NewGasLimitReachedTransitionApplicationError(err)
	}

	// 4. there is no overflow when calculating intrinsic gas
	intrinsicGasCost, err := TransactionGasCost(msg, t.config.Homestead, t.config.Istanbul)
	if err != nil {
		return nil, NewTransitionApplicationError(err, false)
	}

	t.logger.Debug("apply transaction would uses gas", "hash", msg.Hash(), "gas", intrinsicGasCost)

	// 5. the purchased gas is enough to cover intrinsic usage
	gasLeft := msg.Gas - intrinsicGasCost
	// Because we are working with unsigned integers for gas, the `>` operator is used instead of the more intuitive `<`
	if gasLeft > msg.Gas {
		return nil, NewTransitionApplicationError(ErrNotEnoughIntrinsicGas, false)
	}

	// 6. caller has enough balance to cover asset transfer for **topmost** call
	if balance := txn.GetBalance(msg.From); balance.Cmp(msg.Value) < 0 {
		// It is not recoverable. All the transactions after that should be dropped
		return nil, NewTransitionApplicationError(ErrNotEnoughFunds, true)
	}

	gasPrice := new(big.Int).Set(msg.GasPrice)
	value := new(big.Int).Set(msg.Value)

	// Set the specific transaction fields in the context
	t.ctx.GasPrice = types.BytesToHash(gasPrice.Bytes())
	t.ctx.Origin = msg.From

	var result *runtime.ExecutionResult
	if msg.IsContractCreation() {
		result = t.Create2(msg.From, msg.Input, value, gasLeft)
	} else {
		txn.IncrNonce(msg.From)
		result = t.Call2(msg.From, *msg.To, msg.Input, value, gasLeft)
	}

	refund := txn.GetRefund()
	result.UpdateGasUsed(msg.Gas, refund)

	// refund the sender
	remaining := new(big.Int).Mul(new(big.Int).SetUint64(result.GasLeft), gasPrice)
	txn.AddBalance(msg.From, remaining)

	// pay the coinbase
	coinbaseFee := new(big.Int).Mul(new(big.Int).SetUint64(result.GasUsed), gasPrice)
	txn.AddBalance(t.ctx.Coinbase, coinbaseFee)

	// return gas to the pool
	t.addGasPool(result.GasLeft)

	return result, nil
}

func (t *Transition) Create2(
	caller types.Address,
	code []byte,
	value *big.Int,
	gas uint64,
) *runtime.ExecutionResult {
	address := crypto.CreateAddress(caller, t.txn.GetNonce(caller))
	contract := runtime.NewContractCreation(1, caller, caller, address, value, gas, code)

	return t.applyCreate(contract, t)
}

func (t *Transition) Call2(
	caller types.Address,
	to types.Address,
	input []byte,
	value *big.Int,
	gas uint64,
) *runtime.ExecutionResult {
	code := t.txn.GetCode(to)
	c := runtime.NewContractCall(1, caller, caller, to, value, gas, code, input)

	return t.applyCall(c, runtime.Call, t)
}

func (t *Transition) run(contract *runtime.Contract, host runtime.Host) *runtime.ExecutionResult {
	for _, r := range t.r.runtimes {
		if r.CanRun(contract, host, &t.config) {
			return r.Run(contract, host, &t.config)
		}
	}

	return &runtime.ExecutionResult{
		Err: fmt.Errorf("not found"),
	}
}

func (t *Transition) transfer(from, to types.Address, amount *big.Int) error {
	if amount == nil {
		return nil
	}

	if err := t.txn.SubBalance(from, amount); err != nil {
		if errors.Is(err, runtime.ErrNotEnoughFunds) {
			return runtime.ErrInsufficientBalance
		}

		return err
	}

	t.txn.AddBalance(to, amount)

	return nil
}

func (t *Transition) applyCall(
	c *runtime.Contract,
	callType runtime.CallType,
	host runtime.Host,
) *runtime.ExecutionResult {
	if c.Depth > int(1024)+1 {
		return &runtime.ExecutionResult{
			GasLeft: c.Gas,
			Err:     runtime.ErrDepth,
		}
	}

	var result *runtime.ExecutionResult

	if t.needDebug {
		if c.Depth == 0 {
			t.evmLogger.CaptureStart(t.Txn(), c.Caller, c.Address, false, c.Input, c.Gas, c.Value)

			defer func(result *runtime.ExecutionResult) {
				if result != nil {
					t.evmLogger.CaptureEnd(result.ReturnValue, result.GasUsed, time.Since(time.Now()), result.Err)
				}
			}(result)
		} else {
			t.evmLogger.CaptureEnter(int(evm.RuntimeType2OpCode(callType)), c.Caller, c.Address, c.Input, c.Gas, c.Value)

			defer func(result *runtime.ExecutionResult) {
				if result != nil {
					t.evmLogger.CaptureExit(result.ReturnValue, result.GasUsed, result.Err)
				}
			}(result)
		}
	}

	//nolint:ifshort
	snapshot := t.txn.Snapshot()
	t.txn.TouchAccount(c.Address)

	if callType == runtime.Call {
		// Transfers only allowed on calls
		if err := t.transfer(c.Caller, c.Address, c.Value); err != nil {
			result = &runtime.ExecutionResult{
				GasLeft: c.Gas,
				Err:     err,
			}

			return result
		}
	}

	result = t.run(c, host)
	if result.Failed() {
		t.txn.RevertToSnapshot(snapshot)
	}

	return result
}

var emptyHash types.Hash

func (t *Transition) hasCodeOrNonce(addr types.Address) bool {
	nonce := t.txn.GetNonce(addr)
	if nonce != 0 {
		return true
	}

	codeHash := t.txn.GetCodeHash(addr)

	if codeHash != emptyCodeHashTwo && codeHash != emptyHash {
		return true
	}

	return false
}

func (t *Transition) applyCreate(c *runtime.Contract, host runtime.Host) *runtime.ExecutionResult {
	gasLimit := c.Gas

	if c.Depth > int(1024)+1 {
		return &runtime.ExecutionResult{
			GasLeft: gasLimit,
			Err:     runtime.ErrDepth,
		}
	}

	// Increment the nonce of the caller
	t.txn.IncrNonce(c.Caller)

	// Check if there if there is a collision and the address already exists
	if t.hasCodeOrNonce(c.Address) {
		return &runtime.ExecutionResult{
			GasLeft: 0,
			Err:     runtime.ErrContractAddressCollision,
		}
	}

	// Take snapshot of the current state
	snapshot := t.txn.Snapshot()

	if t.config.EIP158 {
		// Force the creation of the account
		t.txn.CreateAccount(c.Address)
		t.txn.IncrNonce(c.Address)
	}

	var result *runtime.ExecutionResult

	if t.needDebug {
		if c.Depth == 0 {
			t.evmLogger.CaptureStart(t.Txn(), c.Caller, c.Address, true, c.Input, c.Gas, c.Value)

			defer func(result *runtime.ExecutionResult) {
				if result != nil {
					t.evmLogger.CaptureEnd(result.ReturnValue, result.GasUsed, time.Since(time.Now()), result.Err)
				}
			}(result)
		} else {
			t.evmLogger.CaptureEnter(int(evm.RuntimeType2OpCode(c.Type)), c.Caller, c.Address, c.Input, c.Gas, c.Value)

			defer func(result *runtime.ExecutionResult) {
				if result != nil {
					t.evmLogger.CaptureExit(result.ReturnValue, result.GasUsed, result.Err)
				}
			}(result)
		}
	}

	// Transfer the value
	if err := t.transfer(c.Caller, c.Address, c.Value); err != nil {
		result = &runtime.ExecutionResult{
			GasLeft: gasLimit,
			Err:     err,
		}

		return result
	}

	result = t.run(c, host)

	if result.Failed() {
		t.txn.RevertToSnapshot(snapshot)

		return result
	}

	if t.config.EIP158 && len(result.ReturnValue) > spuriousDragonMaxCodeSize {
		// Contract size exceeds 'SpuriousDragon' size limit
		t.txn.RevertToSnapshot(snapshot)

		return &runtime.ExecutionResult{
			GasLeft: 0,
			Err:     runtime.ErrMaxCodeSizeExceeded,
		}
	}

	gasCost := uint64(len(result.ReturnValue)) * 200

	if result.GasLeft < gasCost {
		result.Err = runtime.ErrCodeStoreOutOfGas
		result.ReturnValue = nil

		// Out of gas creating the contract
		if t.config.Homestead {
			t.txn.RevertToSnapshot(snapshot)

			result.GasLeft = 0
		}

		return result
	}

	result.GasLeft -= gasCost
	t.txn.SetCode(c.Address, result.ReturnValue)

	return result
}

func (t *Transition) SetStorage(
	addr types.Address,
	key types.Hash,
	value types.Hash,
	config *chain.ForksInTime,
) runtime.StorageStatus {
	return t.txn.SetStorage(addr, key, value, config)
}

func (t *Transition) GetTxContext() runtime.TxContext {
	return t.ctx
}

func (t *Transition) GetBlockHash(number int64) (res types.Hash) {
	return t.getHash(uint64(number))
}

func (t *Transition) EmitLog(addr types.Address, topics []types.Hash, data []byte) {
	t.txn.EmitLog(addr, topics, data)
}

func (t *Transition) GetCodeSize(addr types.Address) int {
	return t.txn.GetCodeSize(addr)
}

func (t *Transition) GetCodeHash(addr types.Address) (res types.Hash) {
	return t.txn.GetCodeHash(addr)
}

func (t *Transition) GetCode(addr types.Address) []byte {
	return t.txn.GetCode(addr)
}

func (t *Transition) GetBalance(addr types.Address) *big.Int {
	return t.txn.GetBalance(addr)
}

func (t *Transition) GetStorage(addr types.Address, key types.Hash) types.Hash {
	return t.txn.GetState(addr, key)
}

func (t *Transition) AccountExists(addr types.Address) bool {
	return t.txn.Exist(addr)
}

func (t *Transition) Empty(addr types.Address) bool {
	return t.txn.Empty(addr)
}

func (t *Transition) GetNonce(addr types.Address) uint64 {
	return t.txn.GetNonce(addr)
}

func (t *Transition) Selfdestruct(addr types.Address, beneficiary types.Address) {
	if !t.txn.HasSuicided(addr) {
		t.txn.AddRefund(24000)
	}

	t.txn.AddBalance(beneficiary, t.txn.GetBalance(addr))
	t.txn.Suicide(addr)
}

func (t *Transition) Callx(c *runtime.Contract, h runtime.Host) *runtime.ExecutionResult {
	if c.Type == runtime.Create {
		return t.applyCreate(c, h)
	}

	return t.applyCall(c, c.Type, h)
}

// SetAccountDirectly sets an account to the given address
// NOTE: SetAccountDirectly changes the world state without a transaction
func (t *Transition) SetAccountDirectly(addr types.Address, account *chain.GenesisAccount) error {
	if t.AccountExists(addr) {
		return fmt.Errorf("can't add account to %+v because an account exists already", addr)
	}

	t.txn.SetCode(addr, account.Code)

	for key, value := range account.Storage {
		t.txn.SetStorage(addr, key, value, &t.config)
	}

	t.txn.SetBalance(addr, account.Balance)
	t.txn.SetNonce(addr, account.Nonce)

	return nil
}

func TransactionGasCost(msg *types.Transaction, isHomestead, isIstanbul bool) (uint64, error) {
	cost := uint64(0)

	// Contract creation is only paid on the homestead fork
	if msg.IsContractCreation() && isHomestead {
		cost += TxGasContractCreation
	} else {
		cost += TxGas
	}

	payload := msg.Input
	if len(payload) > 0 {
		zeros := uint64(0)

		for i := 0; i < len(payload); i++ {
			if payload[i] == 0 {
				zeros++
			}
		}

		nonZeros := uint64(len(payload)) - zeros
		nonZeroCost := uint64(68)

		if isIstanbul {
			nonZeroCost = 16
		}

		if (math.MaxUint64-cost)/nonZeroCost < nonZeros {
			return 0, ErrIntrinsicGasOverflow
		}

		cost += nonZeros * nonZeroCost

		if (math.MaxUint64-cost)/4 < zeros {
			return 0, ErrIntrinsicGasOverflow
		}

		cost += zeros * 4
	}

	return cost, nil
}
