package graphql

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/dogechain-lab/dogechain/graphql/argtype"
	rpc "github.com/dogechain-lab/dogechain/jsonrpc"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/umbracle/fastrlp"
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
		a.blockNrOrHash.BlockNumber, _ = rpc.CreateBlockNumberPointer("latest")
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
	var (
		defaultCount = argtype.Uint64(0)
	)

	root, err := a.getStateRoot(ctx)
	if err != nil {
		return defaultCount, err
	}

	// Extract the account balance
	acc, err := a.backend.GetAccount(root, a.address)
	if errors.Is(err, rpc.ErrStateNotFound) {
		// Account not found, return an empty account
		return defaultCount, nil
	} else if err != nil {
		return defaultCount, err
	}

	return argtype.Uint64(acc.Nonce), nil
}

func (a *Account) Code(ctx context.Context) (argtype.Bytes, error) {
	var (
		defaultCode = argtype.Bytes("0x")
		emptyCode   = argtype.Bytes{}
	)

	root, err := a.getStateRoot(ctx)
	if err != nil {
		return defaultCode, err
	}

	acc, err := a.backend.GetAccount(root, a.address)
	if errors.Is(err, rpc.ErrStateNotFound) {
		// Account not found, return default value
		return defaultCode, nil
	} else if err != nil {
		return emptyCode, err
	}

	code, err := a.backend.GetCode(types.BytesToHash(acc.CodeHash))
	if err != nil {
		return emptyCode, nil
	}

	return argtype.Bytes(code), nil
}

func (a *Account) Storage(ctx context.Context, args struct{ Slot types.Hash }) (types.Hash, error) {
	var (
		defaultHash = types.ZeroHash
	)

	root, err := a.getStateRoot(ctx)
	if err != nil {
		return defaultHash, err
	}

	// Get the storage for the passed in location
	result, err := a.backend.GetStorage(root, a.address, args.Slot)
	if err != nil {
		if errors.Is(err, rpc.ErrStateNotFound) {
			return defaultHash, nil
		}

		return defaultHash, err
	}

	// Parse the RLP value
	p := &fastrlp.Parser{}

	v, err := p.Parse(result)
	if err != nil {
		return defaultHash, nil
	}

	data, err := v.Bytes()
	if err != nil {
		return defaultHash, nil
	}

	return types.BytesToHash(data), nil
}

// Log represents an individual log message. All arguments are mandatory.
type Log struct {
	backend     GraphQLStore
	transaction *Transaction
	log         *types.Log
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
	return int32(0)
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
	// backend GraphQLStore
	hash types.Hash
	// tx   *types.Transaction
	// block   *Block
	// index   uint64
}

// // resolve returns the internal transaction object, fetching it if needed.
// func (t *Transaction) resolve(ctx context.Context) (*types.Transaction, error) {
// 	return t.tx, nil
// }

func (t *Transaction) Hash(ctx context.Context) types.Hash {
	return t.hash
}

func (t *Transaction) InputData(ctx context.Context) (argtype.Bytes, error) {
	return argtype.Bytes{}, nil
}

func (t *Transaction) Gas(ctx context.Context) (argtype.Uint64, error) {
	return argtype.Uint64(0), nil
}

func (t *Transaction) GasPrice(ctx context.Context) (argtype.Big, error) {
	return argtype.Big{}, nil
}

func (t *Transaction) Value(ctx context.Context) (argtype.Big, error) {
	return argtype.Big{}, nil
}

func (t *Transaction) Nonce(ctx context.Context) (argtype.Uint64, error) {
	return argtype.Uint64(0), nil
}

func (t *Transaction) To(ctx context.Context, args BlockNumberArgs) (*Account, error) {
	return &Account{}, nil
}

func (t *Transaction) From(ctx context.Context, args BlockNumberArgs) (*Account, error) {
	return &Account{}, nil
}

func (t *Transaction) Block(ctx context.Context) (*Block, error) {
	return &Block{}, nil
}

func (t *Transaction) Index(ctx context.Context) (*int32, error) {
	var index = int32(0)

	return &index, nil
}

// // getReceipt returns the receipt associated with this transaction, if any.
// func (t *Transaction) getReceipt(ctx context.Context) (*types.Receipt, error) {
// 	return &types.Receipt{}, nil
// }

func (t *Transaction) Status(ctx context.Context) (*argtype.Long, error) {
	var status = argtype.Long(0)

	return &status, nil
}

func (t *Transaction) GasUsed(ctx context.Context) (*argtype.Long, error) {
	var gasUsed = argtype.Long(0)

	return &gasUsed, nil
}

func (t *Transaction) CumulativeGasUsed(ctx context.Context) (*argtype.Long, error) {
	var cumulativeGasUsed = argtype.Long(0)

	return &cumulativeGasUsed, nil
}

func (t *Transaction) CreatedContract(ctx context.Context, args BlockNumberArgs) (*Account, error) {
	return &Account{}, nil
}

func (t *Transaction) Logs(ctx context.Context) (*[]*Log, error) {
	var logs = make([]*Log, 0)

	return &logs, nil
}

func (t *Transaction) R(ctx context.Context) (argtype.Big, error) {
	return argtype.Big{}, nil
}

func (t *Transaction) S(ctx context.Context) (argtype.Big, error) {
	return argtype.Big{}, nil
}

func (t *Transaction) V(ctx context.Context) (argtype.Big, error) {
	return argtype.Big{}, nil
}

func (t *Transaction) Raw(ctx context.Context) (argtype.Bytes, error) {
	return argtype.Bytes{}, nil
}

func (t *Transaction) RawReceipt(ctx context.Context) (argtype.Bytes, error) {
	return argtype.Bytes{}, nil
}

// Block represents an Dogechain block.
// backend, and numberOrHash are mandatory. All other fields are lazily fetched
// when required.
type Block struct {
	// numberOrHash *rpc.BlockNumberOrHash
	// hash         types.Hash
	// header       *types.Header
	// block        *types.Block
	// receipts     []*types.Receipt
}

// // resolve returns the internal Block object representing this block, fetching
// // it if necessary.
// func (b *Block) resolve(ctx context.Context) (*types.Block, error) {
// 	return &types.Block{}, nil
// }

// // resolveHeader returns the internal Header object for this block, fetching it
// // if necessary. Call this function instead of `resolve` unless you need the
// // additional data (transactions and uncles).
// func (b *Block) resolveHeader(ctx context.Context) (*types.Header, error) {
// 	return &types.Header{}, nil
// }

// // resolveReceipts returns the list of receipts for this block, fetching them
// // if necessary.
// func (b *Block) resolveReceipts(ctx context.Context) ([]*types.Receipt, error) {
// 	return []*types.Receipt{}, nil
// }

func (b *Block) Number(ctx context.Context) (argtype.Long, error) {
	return argtype.Long(0), nil
}

func (b *Block) Hash(ctx context.Context) (types.Hash, error) {
	return types.ZeroHash, nil
}

func (b *Block) GasLimit(ctx context.Context) (argtype.Long, error) {
	return argtype.Long(0), nil
}

func (b *Block) GasUsed(ctx context.Context) (argtype.Long, error) {
	return argtype.Long(0), nil
}

func (b *Block) Parent(ctx context.Context) (*Block, error) {
	return &Block{}, nil
}

func (b *Block) Difficulty(ctx context.Context) (argtype.Big, error) {
	return argtype.Big{}, nil
}

func (b *Block) Timestamp(ctx context.Context) (argtype.Uint64, error) {
	return argtype.Uint64(0), nil
}

func (b *Block) Nonce(ctx context.Context) (argtype.Bytes, error) {
	return argtype.Bytes{}, nil
}

func (b *Block) MixHash(ctx context.Context) (types.Hash, error) {
	return types.ZeroHash, nil
}

func (b *Block) TransactionsRoot(ctx context.Context) (types.Hash, error) {
	return types.ZeroHash, nil
}

func (b *Block) StateRoot(ctx context.Context) (types.Hash, error) {
	return types.ZeroHash, nil
}

func (b *Block) ReceiptsRoot(ctx context.Context) (types.Hash, error) {
	return types.ZeroHash, nil
}

func (b *Block) ExtraData(ctx context.Context) (argtype.Bytes, error) {
	return argtype.Bytes{}, nil
}

func (b *Block) LogsBloom(ctx context.Context) (argtype.Bytes, error) {
	return argtype.Bytes{}, nil
}

func (b *Block) TotalDifficulty(ctx context.Context) (argtype.Big, error) {
	return argtype.Big{}, nil
}

func (b *Block) RawHeader(ctx context.Context) (argtype.Bytes, error) {
	return argtype.Bytes{}, nil
}

func (b *Block) Raw(ctx context.Context) (argtype.Bytes, error) {
	return argtype.Bytes{}, nil
}

// BlockNumberArgs encapsulates arguments to accessors that specify a block number.
type BlockNumberArgs struct {
	// TODO: Ideally we could use input unions to allow the query to specify the
	// block parameter by hash, block number, or tag but input unions aren't part of the
	// standard GraphQL schema SDL yet, see: https://github.com/graphql/graphql-spec/issues/488
	Block *argtype.Uint64
}

// NumberOr returns the provided block number argument, or the "current" block number or hash if none
// was provided.
func (a BlockNumberArgs) NumberOr(current rpc.BlockNumberOrHash) rpc.BlockNumberOrHash {
	return rpc.BlockNumberOrHash{}
}

// NumberOrLatest returns the provided block number argument, or the "latest" block number if none
// was provided.
func (a BlockNumberArgs) NumberOrLatest() rpc.BlockNumberOrHash {
	return rpc.BlockNumberOrHash{}
}

func (b *Block) Miner(ctx context.Context, args BlockNumberArgs) (*Account, error) {
	return &Account{}, nil
}

func (b *Block) TransactionCount(ctx context.Context) (*int32, error) {
	var count = int32(0)

	return &count, nil
}

func (b *Block) Transactions(ctx context.Context) (*[]*Transaction, error) {
	return &[]*Transaction{}, nil
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

func (b *Block) Logs(ctx context.Context, args struct{ Filter BlockFilterCriteria }) ([]*Log, error) {
	return []*Log{}, nil
}

func (b *Block) Account(ctx context.Context, args struct {
	Address types.Address
}) (*Account, error) {
	return &Account{}, nil
}

// Resolver is the top-level object in the GraphQL hierarchy.
type Resolver struct {
	backend GraphQLStore
	chainID argtype.Big
}

func (r *Resolver) Block(ctx context.Context, args struct {
	Number *argtype.Long
	Hash   *types.Hash
}) (*Block, error) {
	return &Block{}, nil
}

func (r *Resolver) Blocks(ctx context.Context, args struct {
	From *argtype.Long
	To   *argtype.Long
}) ([]*Block, error) {
	return []*Block{}, nil
}

func (r *Resolver) Transaction(ctx context.Context, args struct{ Hash types.Hash }) (*Transaction, error) {
	return &Transaction{}, nil
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
	return []*Log{}, nil
}

func (r *Resolver) GasPrice(ctx context.Context) (argtype.Big, error) {
	return argtype.Big{}, nil
}

func (r *Resolver) ChainID(ctx context.Context) (argtype.Big, error) {
	return r.chainID, nil
}
