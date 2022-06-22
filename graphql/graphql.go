// TODO: context not used in all graphql query
package graphql

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/dogechain-lab/dogechain/crypto"
	"github.com/dogechain-lab/dogechain/graphql/argtype"
	rpc "github.com/dogechain-lab/dogechain/jsonrpc"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/dogechain-lab/fastrlp"
)

var (
	latestBlockNum, _ = rpc.CreateBlockNumberPointer(rpc.LatestBlockFlag)
)

var (
	errBlockInvariant    = errors.New("block objects must be instantiated with at least one of num or hash")
	errBlockNotExists    = errors.New("block not exists")
	errBlockInvalidRange = errors.New("block range invalid")
)

// Account represents an Dogechain account at a particular block.
type Account struct {
	backend       GraphQLStore
	address       types.Address
	blockNrOrHash rpc.BlockNumberOrHash
}

// getState fetches the StateDB object for a account.
func (a *Account) getStateRoot(ctx context.Context) (types.Hash, error) {
	// The filter is empty, use the latest block by default
	if a.blockNrOrHash.BlockNumber == nil && a.blockNrOrHash.BlockHash == nil {
		a.blockNrOrHash.BlockNumber = latestBlockNum
	}

	header, err := a.getHeaderFromBlockNumberOrHash(&a.blockNrOrHash)
	if err != nil {
		return types.ZeroHash, fmt.Errorf("failed to get header from block hash or block number")
	}

	return header.StateRoot, nil
}

func (a *Account) getHeaderFromBlockNumberOrHash(bnh *rpc.BlockNumberOrHash) (*types.Header, error) {
	var (
		header *types.Header
		err    error
	)

	if bnh.BlockNumber != nil {
		header, err = a.getBlockHeader(*bnh.BlockNumber)
		if err != nil {
			return nil, fmt.Errorf("failed to get the header of block %d: %w", *bnh.BlockNumber, err)
		}
	} else if bnh.BlockHash != nil {
		block, ok := a.backend.GetBlockByHash(*bnh.BlockHash, false)
		if !ok {
			return nil, fmt.Errorf("could not find block referenced by the hash %s", bnh.BlockHash.String())
		}

		header = block.Header
	}

	return header, nil
}

func (a *Account) getBlockHeader(number rpc.BlockNumber) (*types.Header, error) {
	switch number {
	case rpc.LatestBlockNumber:
		return a.backend.Header(), nil

	case rpc.EarliestBlockNumber:
		header, ok := a.backend.GetHeaderByNumber(uint64(0))
		if !ok {
			return nil, fmt.Errorf("error fetching genesis block header")
		}

		return header, nil

	case rpc.PendingBlockNumber:
		return nil, fmt.Errorf("fetching the pending header is not supported")

	default:
		// Convert the block number from hex to uint64
		header, ok := a.backend.GetHeaderByNumber(uint64(number))
		if !ok {
			return nil, fmt.Errorf("error fetching block number %d header", uint64(number))
		}

		return header, nil
	}
}

func (a *Account) Address(ctx context.Context) (types.Address, error) {
	return a.address, nil
}

func (a *Account) Balance(ctx context.Context) (argtype.Big, error) {
	var (
		balance        = big.NewInt(0)
		defaultBalance = argtype.Big(*balance)
	)

	root, err := a.getStateRoot(ctx)
	if err != nil {
		return defaultBalance, err
	}

	// Extract the account balance
	acc, err := a.backend.GetAccount(root, a.address)
	if errors.Is(err, rpc.ErrStateNotFound) {
		// Account not found, return an empty account
		return defaultBalance, nil
	} else if err != nil {
		return defaultBalance, err
	}

	return argtype.Big(*acc.Balance), nil
}

func (a *Account) TransactionCount(ctx context.Context) (argtype.Uint64, error) {
	root, err := a.getStateRoot(ctx)
	if err != nil {
		return 0, err
	}

	// Extract the account balance
	acc, err := a.backend.GetAccount(root, a.address)
	if errors.Is(err, rpc.ErrStateNotFound) {
		// Account not found, return an empty account
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	return argtype.Uint64(acc.Nonce), nil
}

func (a *Account) Code(ctx context.Context) (argtype.Bytes, error) {
	root, err := a.getStateRoot(ctx)
	if err != nil {
		return argtype.Bytes{}, err
	}

	acc, err := a.backend.GetAccount(root, a.address)
	if errors.Is(err, rpc.ErrStateNotFound) {
		// Account not found, return default value
		return argtype.Bytes{}, nil
	} else if err != nil {
		return argtype.Bytes{}, err
	}

	code, err := a.backend.GetCode(types.BytesToHash(acc.CodeHash))
	if err != nil {
		return argtype.Bytes{}, nil
	}

	return argtype.Bytes(code), nil
}

func (a *Account) Storage(ctx context.Context, args struct{ Slot types.Hash }) (types.Hash, error) {
	root, err := a.getStateRoot(ctx)
	if err != nil {
		return types.ZeroHash, err
	}

	// Get the storage for the passed in location
	result, err := a.backend.GetStorage(root, a.address, args.Slot)
	if err != nil {
		if errors.Is(err, rpc.ErrStateNotFound) {
			return types.ZeroHash, nil
		}

		return types.ZeroHash, err
	}

	// Parse the RLP value
	p := &fastrlp.Parser{}

	v, err := p.Parse(result)
	if err != nil {
		return types.ZeroHash, nil
	}

	data, err := v.Bytes()
	if err != nil {
		return types.ZeroHash, nil
	}

	return types.BytesToHash(data), nil
}

// Log represents an individual log message. All arguments are mandatory.
type Log struct {
	backend     GraphQLStore
	transaction *Transaction
	log         *argtype.Log
}

func (l *Log) Transaction(ctx context.Context) *Transaction {
	return l.transaction
}

func (l *Log) Account(ctx context.Context, args BlockNumberArgs) *Account {
	return &Account{
		backend:       l.backend,
		address:       l.log.Address,
		blockNrOrHash: args.NumberOrLatest(),
	}
}

func (l *Log) Index(ctx context.Context) int32 {
	return int32(l.log.LogIndex)
}

func (l *Log) Topics(ctx context.Context) []types.Hash {
	return l.log.Topics
}

func (l *Log) Data(ctx context.Context) argtype.Bytes {
	return l.log.Data
}

// Transaction represents an Dogechain transaction.
// backend and hash are mandatory; all others will be fetched when required.
type Transaction struct {
	backend  GraphQLStore
	resolver *Resolver
	signer   crypto.TxSigner

	hash  types.Hash
	tx    *types.Transaction
	block *Block
	index uint64
}

// resolve returns the internal transaction object, fetching it if needed.
func (t *Transaction) resolve(ctx context.Context) (*types.Transaction, error) {
	if t.tx == nil {
		// 1. Check the chain state for the txn
		if t.findSealedTx(t.hash) {
			return t.tx, nil
		}

		// 2. Check the TxPool for the txn
		if t.findPendingTx(t.hash) {
			return t.tx, nil
		}

		// Transaction not found in state or TxPool
		return nil, nil
	}

	return t.tx, nil
}

// findSealedTx is a helper method for checking the world state for the transaction
// with the provided hash
func (t *Transaction) findSealedTx(hash types.Hash) bool {
	// Check the chain state for the transaction
	blockHash, ok := t.backend.ReadTxLookup(hash)
	if !ok {
		// Block not found in storage
		return false
	}

	block, ok := t.backend.GetBlockByHash(blockHash, true)
	if !ok {
		// Block receipts not found in storage
		return false
	}

	blockNum := rpc.BlockNumber(block.Header.Number)

	t.block = &Block{
		backend:  t.backend,
		resolver: t.resolver,
		numberOrHash: &rpc.BlockNumberOrHash{
			BlockNumber: &blockNum,
			BlockHash:   &blockHash,
		},
		hash:   blockHash,
		header: block.Header,
		block:  block,
	}

	t.signer = crypto.NewSigner(t.backend.GetForksInTime(block.Number()), t.resolver.chainID)

	// Find the transaction within the block
	for idx, txn := range block.Transactions {
		if txn.Hash == hash {
			t.tx = txn
			t.index = uint64(idx)

			return true
		}
	}

	return false
}

// findPendingTx is a helper method for checking the TxPool for the pending transaction
// with the provided hash
func (t *Transaction) findPendingTx(hash types.Hash) bool {
	// Check the TxPool for the transaction if it's pending
	if pendingTx, pendingFound := t.backend.GetPendingTx(hash); pendingFound {
		header := t.backend.Header()
		if header == nil {
			return false
		}

		t.signer = crypto.NewSigner(t.backend.GetForksInTime(header.Number), t.resolver.chainID)
		t.tx = pendingTx

		return true
	}

	// Transaction not found in the TxPool
	return false
}

func (t *Transaction) Hash(ctx context.Context) types.Hash {
	return t.hash
}

func (t *Transaction) InputData(ctx context.Context) (argtype.Bytes, error) {
	tx, err := t.resolve(ctx)
	if err != nil || tx == nil {
		return argtype.Bytes{}, err
	}

	return argtype.Bytes(t.tx.Input), nil
}

func (t *Transaction) Gas(ctx context.Context) (argtype.Uint64, error) {
	tx, err := t.resolve(ctx)
	if err != nil || tx == nil {
		return 0, err
	}

	return argtype.Uint64(tx.Gas), nil
}

func (t *Transaction) GasPrice(ctx context.Context) (argtype.Big, error) {
	tx, err := t.resolve(ctx)
	if err != nil || tx == nil {
		return argtype.Big{}, err
	}

	if tx.Value == nil {
		return argtype.Big{}, fmt.Errorf("invalid transaction value %x", t.hash)
	}

	return argtype.Big(*tx.GasPrice), nil
}

func (t *Transaction) Value(ctx context.Context) (argtype.Big, error) {
	tx, err := t.resolve(ctx)
	if err != nil || tx == nil || tx.Value == nil {
		return argtype.Big{}, err
	}

	return argtype.Big(*tx.Value), nil
}

func (t *Transaction) Nonce(ctx context.Context) (argtype.Uint64, error) {
	tx, err := t.resolve(ctx)
	if err != nil || tx == nil {
		return 0, err
	}

	return argtype.Uint64(t.tx.Nonce), nil
}

func (t *Transaction) To(ctx context.Context, args BlockNumberArgs) (*Account, error) {
	tx, err := t.resolve(ctx)
	if err != nil || tx == nil {
		return nil, err
	}

	to := tx.To
	if to == nil {
		return nil, nil
	}

	return &Account{
		backend:       t.backend,
		address:       *to,
		blockNrOrHash: args.NumberOrLatest(),
	}, nil
}

func (t *Transaction) From(ctx context.Context, args BlockNumberArgs) (*Account, error) {
	tx, err := t.resolve(ctx)
	if err != nil || tx == nil {
		return nil, err
	}

	// check signer
	from, err := t.signer.Sender(tx)
	if err != nil {
		return nil, err
	}

	return &Account{
		backend:       t.backend,
		address:       from,
		blockNrOrHash: args.NumberOrLatest(),
	}, nil
}

func (t *Transaction) Block(ctx context.Context) (*Block, error) {
	if _, err := t.resolve(ctx); err != nil {
		return nil, err
	}

	return t.block, nil
}

func (t *Transaction) Index(ctx context.Context) (*int32, error) {
	if _, err := t.resolve(ctx); err != nil {
		return nil, err
	}

	if t.block == nil {
		return nil, nil
	}

	var index = int32(t.index)

	return &index, nil
}

// getReceipt returns the receipt associated with this transaction, if any.
func (t *Transaction) getReceipt(ctx context.Context) (*types.Receipt, error) {
	if _, err := t.resolve(ctx); err != nil {
		return nil, err
	}

	if t.block == nil {
		return nil, nil
	}

	receipts, err := t.block.resolveReceipts(ctx)
	if err != nil {
		return nil, err
	}

	return receipts[t.index], nil
}

func (t *Transaction) Status(ctx context.Context) (*argtype.Long, error) {
	receipt, err := t.getReceipt(ctx)
	if err != nil || receipt == nil {
		return nil, err
	}

	if receipt.Status == nil {
		return nil, nil
	}

	ret := argtype.Long(*receipt.Status)

	return &ret, nil
}

func (t *Transaction) GasUsed(ctx context.Context) (*argtype.Long, error) {
	receipt, err := t.getReceipt(ctx)
	if err != nil || receipt == nil {
		return nil, err
	}

	ret := argtype.Long(receipt.GasUsed)

	return &ret, nil
}

func (t *Transaction) CumulativeGasUsed(ctx context.Context) (*argtype.Long, error) {
	receipt, err := t.getReceipt(ctx)
	if err != nil || receipt == nil {
		return nil, err
	}

	ret := argtype.Long(receipt.CumulativeGasUsed)

	return &ret, nil
}

func (t *Transaction) CreatedContract(ctx context.Context, args BlockNumberArgs) (*Account, error) {
	receipt, err := t.getReceipt(ctx)
	if err != nil || receipt == nil ||
		receipt.ContractAddress == nil || *receipt.ContractAddress == types.ZeroAddress {
		return nil, err
	}

	return &Account{
		backend:       t.backend,
		address:       *receipt.ContractAddress,
		blockNrOrHash: args.NumberOrLatest(),
	}, nil
}

func (t *Transaction) Logs(ctx context.Context) (*[]*Log, error) {
	receipt, err := t.getReceipt(ctx)
	if err != nil || receipt == nil {
		return nil, err
	}

	logs := make([]*Log, len(receipt.Logs))
	for i, elem := range receipt.Logs {
		logs[i] = &Log{
			backend:     t.backend,
			transaction: t,
			log: &argtype.Log{
				Address:     elem.Address,
				Topics:      elem.Topics,
				Data:        elem.Data,
				BlockNumber: argtype.Uint64(t.block.header.Number),
				TxHash:      t.hash,
				TxIndex:     argtype.Uint64(t.index),
				BlockHash:   t.block.hash,
				LogIndex:    argtype.Uint64(i),
				Removed:     false,
			},
		}
	}

	return &logs, nil
}

func (t *Transaction) R(ctx context.Context) (argtype.Big, error) {
	tx, err := t.resolve(ctx)
	if err != nil || tx == nil {
		return argtype.Big{}, err
	}

	return argtype.Big(*tx.R), nil
}

func (t *Transaction) S(ctx context.Context) (argtype.Big, error) {
	tx, err := t.resolve(ctx)
	if err != nil || tx == nil {
		return argtype.Big{}, err
	}

	return argtype.Big(*tx.S), nil
}

func (t *Transaction) V(ctx context.Context) (argtype.Big, error) {
	tx, err := t.resolve(ctx)
	if err != nil || tx == nil {
		return argtype.Big{}, err
	}

	return argtype.Big(*tx.V), nil
}

func (t *Transaction) Raw(ctx context.Context) (argtype.Bytes, error) {
	tx, err := t.resolve(ctx)
	if err != nil || tx == nil {
		return argtype.Bytes{}, err
	}

	return argtype.Bytes(tx.MarshalRLP()), nil
}

func (t *Transaction) RawReceipt(ctx context.Context) (argtype.Bytes, error) {
	receipt, err := t.getReceipt(ctx)
	if err != nil || receipt == nil {
		return argtype.Bytes{}, err
	}

	return receipt.MarshalRLP(), nil
}

// Block represents an Dogechain block.
// backend, and numberOrHash are mandatory. All other fields are lazily fetched
// when required.
type Block struct {
	backend  GraphQLStore
	resolver *Resolver

	numberOrHash *rpc.BlockNumberOrHash
	hash         types.Hash
	header       *types.Header
	block        *types.Block
	receipts     []*types.Receipt
}

// resolve returns the internal Block object representing this block, fetching
// it if necessary.
func (b *Block) resolve(ctx context.Context) (*types.Block, error) {
	if b.block != nil {
		return b.block, nil
	}

	if b.numberOrHash == nil {
		b.numberOrHash = &rpc.BlockNumberOrHash{
			BlockNumber: latestBlockNum,
		}
	}

	var exists bool

	switch {
	case b.numberOrHash.BlockNumber != nil:
		// get block with all transactions
		b.block, exists = b.backend.GetBlockByNumber(uint64(*b.numberOrHash.BlockNumber), true)
		if !exists {
			return nil, errBlockNotExists
		}
	case b.numberOrHash.BlockHash != nil:
		b.block, exists = b.backend.GetBlockByHash(*b.numberOrHash.BlockHash, true)
		if !exists {
			return nil, errBlockNotExists
		}

		b.hash = *b.numberOrHash.BlockHash
	default:
		return nil, errBlockInvariant
	}

	if b.block != nil && b.header == nil {
		b.header = b.block.Header

		if b.hash == types.ZeroHash {
			b.hash = b.block.Hash()
		}
	}

	return b.block, nil
}

// resolveHeader returns the internal Header object for this block, fetching it
// if necessary. Call this function instead of `resolve` unless you need the
// additional data (transactions and uncles).
func (b *Block) resolveHeader(ctx context.Context) (*types.Header, error) {
	if b.numberOrHash == nil && b.hash == types.ZeroHash {
		return nil, errBlockInvariant
	}

	if b.header == nil {
		var exists bool

		if b.hash != types.ZeroHash {
			b.header, exists = b.backend.GetHeaderByHash(b.hash)
			if !exists {
				return nil, errBlockNotExists
			}
		} else {
			b.header, exists = b.backend.GetHeaderByNumber(uint64(*b.numberOrHash.BlockNumber))
			if !exists {
				return nil, errBlockNotExists
			}

			b.hash = b.header.Hash
		}
	}

	return b.header, nil
}

// resolveReceipts returns the list of receipts for this block, fetching them if necessary.
func (b *Block) resolveReceipts(ctx context.Context) ([]*types.Receipt, error) {
	if b.receipts == nil {
		hash := b.hash

		if hash == types.ZeroHash {
			header, err := b.resolveHeader(ctx)
			if err != nil {
				return nil, err
			}

			hash = header.Hash
		}

		receipts, err := b.backend.GetReceiptsByHash(hash)
		if err != nil {
			return nil, err
		}

		b.receipts = receipts
	}

	return b.receipts, nil
}

func (b *Block) Number(ctx context.Context) (argtype.Long, error) {
	if _, err := b.resolveHeader(ctx); err != nil {
		return 0, err
	}

	return argtype.Long(b.header.Number), nil
}

func (b *Block) Hash(ctx context.Context) (types.Hash, error) {
	if _, err := b.resolveHeader(ctx); err != nil {
		return types.ZeroHash, err
	}

	return b.hash, nil
}

func (b *Block) GasLimit(ctx context.Context) (argtype.Long, error) {
	if _, err := b.resolveHeader(ctx); err != nil {
		return 0, err
	}

	return argtype.Long(b.header.GasLimit), nil
}

func (b *Block) GasUsed(ctx context.Context) (argtype.Long, error) {
	if _, err := b.resolveHeader(ctx); err != nil {
		return 0, err
	}

	return argtype.Long(b.header.GasUsed), nil
}

func (b *Block) Parent(ctx context.Context) (*Block, error) {
	if _, err := b.resolveHeader(ctx); err != nil {
		return nil, err
	}

	if b.header == nil || b.header.Number == 0 {
		return nil, nil
	}

	// parent number
	num := rpc.BlockNumber(b.header.Number - 1)

	return &Block{
		backend: b.backend,
		numberOrHash: &rpc.BlockNumberOrHash{
			BlockNumber: &num,
		},
		hash: b.header.ParentHash,
	}, nil
}

func (b *Block) Difficulty(ctx context.Context) (argtype.Big, error) {
	if _, err := b.resolveHeader(ctx); err != nil {
		return argtype.Big{}, err
	}

	difficulty := new(big.Int).SetUint64(b.header.Difficulty)

	return argtype.Big(*difficulty), nil
}

func (b *Block) Timestamp(ctx context.Context) (argtype.Uint64, error) {
	if _, err := b.resolveHeader(ctx); err != nil {
		return 0, err
	}

	return argtype.Uint64(b.header.Timestamp), nil
}

func (b *Block) Nonce(ctx context.Context) (argtype.Bytes, error) {
	if _, err := b.resolveHeader(ctx); err != nil {
		return argtype.Bytes{}, err
	}

	return b.header.Nonce[:], nil
}

func (b *Block) MixHash(ctx context.Context) (types.Hash, error) {
	if _, err := b.resolveHeader(ctx); err != nil {
		return types.ZeroHash, err
	}

	return b.header.MixHash, nil
}

func (b *Block) TransactionsRoot(ctx context.Context) (types.Hash, error) {
	if _, err := b.resolveHeader(ctx); err != nil {
		return types.ZeroHash, err
	}

	return b.header.TxRoot, nil
}

func (b *Block) StateRoot(ctx context.Context) (types.Hash, error) {
	if _, err := b.resolveHeader(ctx); err != nil {
		return types.ZeroHash, err
	}

	return b.header.StateRoot, nil
}

func (b *Block) ReceiptsRoot(ctx context.Context) (types.Hash, error) {
	if _, err := b.resolveHeader(ctx); err != nil {
		return types.ZeroHash, err
	}

	return b.header.ReceiptsRoot, nil
}

func (b *Block) ExtraData(ctx context.Context) (argtype.Bytes, error) {
	if _, err := b.resolveHeader(ctx); err != nil {
		return argtype.Bytes{}, err
	}

	return b.header.ExtraData, nil
}

func (b *Block) LogsBloom(ctx context.Context) (argtype.Bytes, error) {
	if _, err := b.resolveHeader(ctx); err != nil {
		return argtype.Bytes{}, err
	}

	return b.header.LogsBloom[:], nil
}

func (b *Block) RawHeader(ctx context.Context) (argtype.Bytes, error) {
	if _, err := b.resolveHeader(ctx); err != nil {
		return argtype.Bytes{}, err
	}

	return b.header.MarshalRLP(), nil
}

func (b *Block) Raw(ctx context.Context) (argtype.Bytes, error) {
	if _, err := b.resolve(ctx); err != nil {
		return argtype.Bytes{}, err
	}

	return b.block.MarshalRLP(), nil
}

// BlockNumberArgs encapsulates arguments to accessors that specify a block number.
type BlockNumberArgs struct {
	Block *argtype.Uint64
}

// NumberOr returns the provided block number argument, or the "current" block number or hash if none
// was provided.
func (a BlockNumberArgs) NumberOr(current rpc.BlockNumberOrHash) rpc.BlockNumberOrHash {
	if a.Block != nil {
		blockNr := rpc.BlockNumber(*a.Block)

		return rpc.BlockNumberOrHash{
			BlockNumber: &blockNr,
			BlockHash:   nil,
		}
	}

	return current
}

// NumberOrLatest returns the provided block number argument, or the "latest" block number if none
// was provided.
func (a BlockNumberArgs) NumberOrLatest() rpc.BlockNumberOrHash {
	return a.NumberOr(rpc.BlockNumberOrHash{
		BlockNumber: latestBlockNum,
		BlockHash:   nil,
	})
}

func (b *Block) Miner(ctx context.Context, args BlockNumberArgs) (*Account, error) {
	if _, err := b.resolveHeader(ctx); err != nil {
		return nil, err
	}

	return &Account{
		backend:       b.backend,
		address:       b.header.Miner,
		blockNrOrHash: args.NumberOrLatest(),
	}, nil
}

func (b *Block) TransactionCount(ctx context.Context) (*int32, error) {
	if _, err := b.resolve(ctx); err != nil {
		return nil, err
	}

	var count = int32(len(b.block.Transactions))

	return &count, nil
}

func (b *Block) Transactions(ctx context.Context) (*[]*Transaction, error) {
	if _, err := b.resolve(ctx); err != nil || b.block == nil {
		return nil, err
	}

	ret := make([]*Transaction, 0, len(b.block.Transactions))
	for i, tx := range b.block.Transactions {
		ret = append(ret, &Transaction{
			backend:  b.backend,
			resolver: b.resolver,
			hash:     tx.Hash,
			tx:       tx,
			block:    b,
			index:    uint64(i),
		})
	}

	return &ret, nil
}

func (b *Block) TransactionAt(ctx context.Context, args struct{ Index int32 }) (*Transaction, error) {
	if _, err := b.resolve(ctx); err != nil || b.block == nil {
		return nil, err
	}

	txs := b.block.Transactions
	if args.Index < 0 || int(args.Index) >= len(txs) {
		return nil, nil
	}

	tx := txs[args.Index]

	return &Transaction{
		backend:  b.backend,
		resolver: b.resolver,
		hash:     tx.Hash,
		tx:       tx,
		block:    b,
		index:    uint64(args.Index),
	}, nil
}

// BlockFilterCriteria encapsulates criteria passed to a `logs` accessor inside
// a block.
type BlockFilterCriteria struct {
	Addresses *[]types.Address // restricts matches to events created by specific contracts

	// The Topic list restricts matches to particular event topics. Each event has a list
	// of topics. Topics matches a prefix of that list. An empty element slice matches any
	// topic. Non-empty elements represent an alternative that matches any of the
	// contained topics.
	//
	// Examples:
	// {} or nil          matches any topic list
	// {{A}}              matches topic A in first position
	// {{}, {B}}          matches any topic in first position, B in second position
	// {{A}, {B}}         matches topic A in first position, B in second position
	// {{A, B}}, {C, D}}  matches topic (A OR B) in first position, (C OR D) in second position
	Topics *[][]types.Hash
}

// runFilter accepts a filter and executes it, returning all its results as
// `Log` objects.
func runFilter(
	ctx context.Context,
	backend GraphQLStore,
	resolver *Resolver,
	filter *rpc.FilterManager,
	query *rpc.LogQuery,
) ([]*Log, error) {
	logs, err := filter.GetLogs(query)
	if err != nil || logs == nil {
		return nil, err
	}

	ret := make([]*Log, 0, len(logs))

	for _, log := range logs {
		t := &Transaction{backend: backend, resolver: resolver, hash: log.TxHash}
		if _, err := t.resolve(ctx); err != nil {
			return nil, err
		}

		ret = append(ret, &Log{
			backend:     backend,
			transaction: t,
			log:         argtype.FromRPCLog(log),
		})
	}

	return ret, nil
}

func (b *Block) Logs(ctx context.Context, args struct{ Filter BlockFilterCriteria }) ([]*Log, error) {
	var addresses []types.Address
	if args.Filter.Addresses != nil {
		addresses = *args.Filter.Addresses
	}

	var topics [][]types.Hash
	if args.Filter.Topics != nil {
		topics = *args.Filter.Topics
	}

	if _, err := b.resolveHeader(ctx); err != nil {
		return nil, err
	}

	num := rpc.BlockNumber(b.header.Number)

	// Run the filter and return all the logs
	return runFilter(ctx, b.backend, b.resolver, b.resolver.filterManager, &rpc.LogQuery{
		FromBlock: num,
		ToBlock:   num,
		Addresses: addresses,
		Topics:    topics,
	})
}

func (b *Block) Account(ctx context.Context, args struct {
	Address types.Address
}) (*Account, error) {
	if _, err := b.resolveHeader(ctx); err != nil {
		return nil, err
	}

	return &Account{
		backend:       b.backend,
		address:       args.Address,
		blockNrOrHash: *b.numberOrHash,
	}, nil
}

// Resolver is the top-level object in the GraphQL hierarchy.
type Resolver struct {
	backend       GraphQLStore
	chainID       uint64
	filterManager *rpc.FilterManager
}

func (r *Resolver) Block(ctx context.Context, args struct {
	Number *argtype.Long
	Hash   *types.Hash
}) (*Block, error) {
	var block *Block

	switch {
	case args.Number != nil:
		if *args.Number < 0 {
			return nil, nil
		}

		number := rpc.BlockNumber(*args.Number)
		numberOrHash := rpc.BlockNumberOrHash{BlockNumber: &number}
		block = &Block{
			backend:      r.backend,
			resolver:     r,
			numberOrHash: &numberOrHash,
		}
	case args.Hash != nil:
		numberOrHash := rpc.BlockNumberOrHash{BlockHash: args.Hash}
		block = &Block{
			backend:      r.backend,
			resolver:     r,
			numberOrHash: &numberOrHash,
		}
	default:
		numberOrHash := rpc.BlockNumberOrHash{BlockNumber: latestBlockNum}
		block = &Block{
			backend:      r.backend,
			resolver:     r,
			numberOrHash: &numberOrHash,
		}
	}

	// Resolve the header, return nil if it doesn't exist.
	// Note we don't resolve block directly here since it will require an
	// additional network request for light client.
	h, err := block.resolveHeader(ctx)
	if err != nil {
		return nil, err
	} else if h == nil {
		return nil, nil
	}

	return block, nil
}

func (r *Resolver) Blocks(ctx context.Context, args struct {
	From *argtype.Long
	To   *argtype.Long
}) ([]*Block, error) {
	var (
		from = rpc.BlockNumber(*args.From)
		to   rpc.BlockNumber
	)

	if args.To != nil {
		to = rpc.BlockNumber(*args.To)
	} else {
		to = rpc.BlockNumber(int64(r.backend.Header().Number))
	}

	if to < from {
		return []*Block{}, nil
	}

	ret := make([]*Block, 0, to-from+1)

	for i := from; i <= to; i++ {
		idx := i // variable copy

		numberOrHash := rpc.BlockNumberOrHash{BlockNumber: &idx}
		block := &Block{
			backend:      r.backend,
			resolver:     r,
			numberOrHash: &numberOrHash,
		}

		// Resolve the header to check for existence.
		// Note we don't resolve block directly here since it will require an
		// additional network request for light client.
		h, err := block.resolveHeader(ctx)
		if err != nil {
			return nil, err
		} else if h == nil {
			// Blocks after must be non-existent too, break.
			break
		}

		ret = append(ret, block)
	}

	return ret, nil
}

func (r *Resolver) Transaction(ctx context.Context, args struct{ Hash types.Hash }) (*Transaction, error) {
	tx := &Transaction{
		backend:  r.backend,
		resolver: r,
		hash:     args.Hash,
	}

	// Resolve the transaction; if it doesn't exist, return nil.
	t, err := tx.resolve(ctx)
	if err != nil {
		return nil, err
	} else if t == nil {
		return nil, nil
	}

	return tx, nil
}

// FilterCriteria encapsulates the arguments to `logs` on the root resolver object.
type FilterCriteria struct {
	FromBlock *argtype.Uint64  // beginning of the queried range, nil means genesis block
	ToBlock   *argtype.Uint64  // end of the range, nil means latest block
	Addresses *[]types.Address // restricts matches to events created by specific contracts

	// The Topic list restricts matches to particular event topics. Each event has a list
	// of topics. Topics matches a prefix of that list. An empty element slice matches any
	// topic. Non-empty elements represent an alternative that matches any of the
	// contained topics.
	//
	// Examples:
	// {} or nil          matches any topic list
	// {{A}}              matches topic A in first position
	// {{}, {B}}          matches any topic in first position, B in second position
	// {{A}, {B}}         matches topic A in first position, B in second position
	// {{A, B}}, {C, D}}  matches topic (A OR B) in first position, (C OR D) in second position
	Topics *[][]types.Hash
}

func (r *Resolver) Logs(ctx context.Context, args struct{ Filter FilterCriteria }) ([]*Log, error) {
	// Convert the RPC block numbers into internal representations
	var (
		begin     = rpc.LatestBlockNumber
		end       = rpc.LatestBlockNumber
		addresses []types.Address
		topics    [][]types.Hash
	)

	if args.Filter.FromBlock != nil {
		begin = rpc.BlockNumber(*args.Filter.FromBlock)
	}

	if args.Filter.ToBlock != nil {
		end = rpc.BlockNumber(*args.Filter.ToBlock)
	}

	if end < begin {
		return nil, errBlockInvalidRange
	}

	if args.Filter.Addresses != nil {
		addresses = *args.Filter.Addresses
	}

	if args.Filter.Topics != nil {
		topics = *args.Filter.Topics
	}

	return runFilter(ctx, r.backend, r, r.filterManager, &rpc.LogQuery{
		FromBlock: begin,
		ToBlock:   end,
		Addresses: addresses,
		Topics:    topics,
	})
}

const (
	defaultMinGasPrice = "0Xba43b7400" // 50 GWei
)

func (r *Resolver) GasPrice(ctx context.Context) (argtype.Big, error) {
	var avgGasPrice *big.Int

	// Grab the average gas price and convert it to a hex value
	minGasPrice, _ := new(big.Int).SetString(defaultMinGasPrice, 0)

	if r.backend.GetAvgGasPrice().Cmp(minGasPrice) == -1 {
		avgGasPrice = minGasPrice
	} else {
		avgGasPrice = r.backend.GetAvgGasPrice()
	}

	if avgGasPrice == nil {
		return argtype.Big{}, nil
	}

	return argtype.Big(*avgGasPrice), nil
}

func (r *Resolver) ChainID(ctx context.Context) (argtype.Big, error) {
	return argtype.Big(*new(big.Int).SetUint64(r.chainID)), nil
}
