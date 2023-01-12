package txpool

import (
	"github.com/dogechain-lab/dogechain/types"
)

/* QUERY methods */
// Used to query the pool for specific state info.

// GetNonce returns the next nonce for the account
//
// -> Returns the value from the TxPool if the account is initialized in-memory
//
// -> Returns the value from the world state otherwise
func (p *TxPool) GetNonce(addr types.Address) uint64 {
	account := p.accounts.get(addr)
	if account == nil {
		stateRoot := p.store.Header().StateRoot
		stateNonce := p.store.GetNonce(stateRoot, addr)

		return stateNonce
	}

	return account.getNonce()
}

// GetCapacity returns the current number of slots
// occupied in the pool as well as the max limit
func (p *TxPool) GetCapacity() (uint64, uint64) {
	return p.gauge.read(), p.gauge.max
}

// GetPendingTx returns the transaction by hash in the TxPool (pending txn) [Thread-safe]
func (p *TxPool) GetPendingTx(txHash types.Hash) (*types.Transaction, bool) {
	tx, ok := p.index.get(txHash)
	if !ok {
		return nil, false
	}

	return tx, true
}

// GetTxs gets pending and queued transactions
func (p *TxPool) GetTxs(inclQueued bool) (
	allPromoted, allEnqueued map[types.Address][]*types.Transaction,
) {
	allPromoted, allEnqueued = p.accounts.allTxs(inclQueued)

	return
}

func (p *TxPool) Pending() map[types.Address][]*types.Transaction {
	return p.accounts.poolPendings()
}

const (
	DDosWhiteList = "whitelist"
	DDosBlackList = "blacklist"
)

// GetDDosContractList shows current white list and black list contracts
func (p *TxPool) GetDDosContractList() map[string]map[types.Address]struct{} {
	var (
		ret    = make(map[string]map[types.Address]struct{}, 2)
		blacks = make(map[types.Address]struct{})
		whites = make(map[types.Address]struct{})
	)

	p.ddosContracts.Range(func(key, value interface{}) bool {
		addr, _ := key.(types.Address)
		count, _ := value.(int)

		if isCountExceedDDOSLimit(count) {
			blacks[addr] = struct{}{}
		}

		return true
	})

	p.ddosWhiteList.Range(func(key, value interface{}) bool {
		addr, _ := key.(types.Address)
		whites[addr] = struct{}{}

		return true
	})

	ret[DDosBlackList] = blacks
	ret[DDosWhiteList] = whites

	return ret
}
