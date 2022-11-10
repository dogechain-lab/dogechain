package txpool

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dogechain-lab/dogechain/types"
)

// Thread safe map of all accounts registered by the pool.
// Each account (value) is bound to one address (key).
type accountsMap struct {
	// sync.Map is thread-safe and high performance.
	// We should not use some simple locked implementation map for this complicate usage,
	// otherwise there might be deadlock.
	//
	// PS: accountsMap is never clear
	// so golang#40999 (https://github.com/golang/go/issues/40999) is no problem,
	// the price is high memory footprint
	// but problem in restore blockchain from snapshot will cause high memory use (never release).
	cmap  sync.Map
	count uint64
}

func newAccountsMap() *accountsMap {
	return &accountsMap{}
}

// Intializes an account for the given address.
func (m *accountsMap) initOnce(addr types.Address, nonce uint64) *account {
	a, _ := m.cmap.LoadOrStore(addr, &account{})
	newAccount := a.(*account) //nolint:forcetypeassert
	// run only once
	newAccount.init.Do(func() {
		// create queues
		newAccount.enqueued = newAccountQueue()
		newAccount.promoted = newAccountQueue()

		// set the nonce
		newAccount.setNonce(nonce)

		// set the timestamp for pruning. Reinit account should reset it.
		newAccount.updatePromoted()

		// update global count
		atomic.AddUint64(&m.count, 1)
	})

	return newAccount
}

// exists checks if an account exists within the map.
func (m *accountsMap) exists(addr types.Address) bool {
	_, ok := m.cmap.Load(addr)

	return ok
}

// getPrimaries collects the heads (first-in-line transaction)
// from each of the promoted queues.
func (m *accountsMap) getPrimaries() (primaries []*types.Transaction) {
	m.cmap.Range(func(key, value interface{}) bool {
		addressKey, ok := key.(types.Address)
		if !ok {
			return false
		}

		account := m.get(addressKey)

		account.promoted.lock(false)
		defer account.promoted.unlock()

		// add head of the queue
		if tx := account.promoted.peek(); tx != nil {
			primaries = append(primaries, tx)
		}

		return true
	})

	return primaries
}

// get returns the account associated with the given address.
func (m *accountsMap) get(addr types.Address) *account {
	a, ok := m.cmap.Load(addr)
	if !ok {
		return nil
	}

	fetchedAccount, ok := a.(*account)
	if !ok {
		return nil
	}

	return fetchedAccount
}

// promoted returns the number of all promoted transactons.
func (m *accountsMap) promoted() (total uint64) {
	m.cmap.Range(func(key, value interface{}) bool {
		accountKey, ok := key.(types.Address)
		if !ok {
			return false
		}

		account := m.get(accountKey)

		account.promoted.lock(false)
		defer account.promoted.unlock()

		total += account.promoted.length()

		return true
	})

	return
}

// promoted returns the number of all promoted transactons.
func (m *accountsMap) enqueued() (total uint64) {
	m.cmap.Range(func(key, value interface{}) bool {
		accountKey, ok := key.(types.Address)
		if !ok {
			return false
		}

		account := m.get(accountKey)

		account.enqueued.lock(false)
		defer account.enqueued.unlock()

		total += account.enqueued.length()

		return true
	})

	return
}

// allTxs returns all promoted and all enqueued transactions, depending on the flag.
func (m *accountsMap) allTxs(includeEnqueued bool) (
	allPromoted, allEnqueued map[types.Address][]*types.Transaction,
) {
	allPromoted = make(map[types.Address][]*types.Transaction)
	allEnqueued = make(map[types.Address][]*types.Transaction)

	m.cmap.Range(func(key, value interface{}) bool {
		addr, _ := key.(types.Address)
		account := m.get(addr)

		account.promoted.lock(false)
		defer account.promoted.unlock()

		if account.promoted.length() != 0 {
			allPromoted[addr] = account.promoted.Transactions()
		}

		if includeEnqueued {
			account.enqueued.lock(false)
			defer account.enqueued.unlock()

			if account.enqueued.length() != 0 {
				allEnqueued[addr] = account.enqueued.Transactions()
			}
		}

		return true
	})

	return
}

func (m *accountsMap) pruneStaleEnqueuedTxs(outdateDuration time.Duration) []*types.Transaction {
	var (
		pruned = make([]*types.Transaction, 0)
		// use same time for faster comparison
		outdateTimeBound = time.Now().Add(-1 * outdateDuration)
	)

	m.cmap.Range(func(_, value interface{}) bool {
		account, ok := value.(*account)
		if !ok {
			// It shouldn't be. We just do some prevention work.
			return false
		}
		// should not do anything, make things faster
		if account.enqueued.length() == 0 {
			return true
		}

		if account.IsOutdated(outdateTimeBound) {
			// only lock the account when needed
			account.enqueued.lock(true)
			pruned = append(
				pruned,
				account.enqueued.Clear()...,
			)
			account.enqueued.unlock()
		}

		return true
	})

	return pruned
}

// poolPendings returns all promoted nonce ascending transactions.
func (m *accountsMap) poolPendings() map[types.Address][]*types.Transaction {
	allPromoted := make(map[types.Address][]*types.Transaction)

	m.cmap.Range(func(key, value interface{}) bool {
		addr, _ := key.(types.Address)
		account := m.get(addr)

		account.promoted.lock(false)
		defer account.promoted.unlock()

		if account.promoted.length() != 0 {
			allPromoted[addr] = account.promoted.Transactions()
		}

		sort.Stable(types.PoolTxByNonce(allPromoted[addr]))

		return true
	})

	return allPromoted
}

// An account is the core structure for processing
// transactions from a specific address. The nextNonce
// field is what separates the enqueued from promoted transactions:
//
// 1. enqueued - transactions higher than the nextNonce
// 2. promoted - transactions lower than the nextNonce
//
// If an enqueued transaction matches the nextNonce,
// a promoteRequest is signaled for this account
// indicating the account's enqueued transaction(s)
// are ready to be moved to the promoted queue.
//
// If an account is not promoted for a long time, its enqueued transactions
// should be remove to reduce txpool stress.
type account struct {
	init               sync.Once
	enqueued, promoted *accountQueue
	nextNonce          uint64
	lastPromoted       time.Time // timestamp for pruning
}

// getNonce returns the next expected nonce for this account.
func (a *account) getNonce() uint64 {
	return atomic.LoadUint64(&a.nextNonce)
}

// setNonce sets the next expected nonce for this account.
func (a *account) setNonce(nonce uint64) {
	atomic.StoreUint64(&a.nextNonce, nonce)
}

// reset aligns the account with the new nonce
// by pruning all transactions with nonce lesser than new.
// After pruning, a promotion may be signaled if the first
// enqueued transaction matches the new nonce.
func (a *account) reset(nonce uint64, promoteCh chan<- promoteRequest) (
	prunedPromoted,
	prunedEnqueued []*types.Transaction,
) {
	a.promoted.lock(true)
	defer a.promoted.unlock()

	//	prune the promoted txs
	prunedPromoted = append(
		prunedPromoted,
		a.promoted.prune(nonce)...,
	)

	if nonce <= a.getNonce() {
		// only the promoted queue needed pruning
		return
	}

	a.enqueued.lock(true)
	defer a.enqueued.unlock()

	//	prune the enqueued txs
	prunedEnqueued = append(
		prunedEnqueued,
		a.enqueued.prune(nonce)...,
	)

	//	update nonce expected for this account
	a.setNonce(nonce)

	//	it is important to signal promotion while
	//	the locks are held to ensure no other
	//	handler will mutate the account
	if first := a.enqueued.peek(); first != nil &&
		first.Nonce == nonce {
		// first enqueued tx is expected -> signal promotion
		promoteCh <- promoteRequest{account: first.From}
	}

	return
}

// enqueue attempts tp push the transaction onto the enqueued queue.
func (a *account) enqueue(tx *types.Transaction) (oldTx *types.Transaction, err error) {
	// find out the same nonce transaction in all queues
	replacable, oldTx := a.enqueued.SameNonceTx(tx)
	if !replacable && oldTx == nil {
		// find it in promoted queue when enqueued queue not found
		replacable, oldTx = a.promoted.SameNonceTx(tx)
	}

	if !replacable {
		if oldTx != nil {
			return nil, ErrReplaceUnderpriced
		}

		// check nonce
		if tx.Nonce < a.getNonce() {
			return nil, ErrNonceTooLow
		}
	}

	// only lock the queue when adding
	a.enqueued.lock(true)
	defer a.enqueued.unlock()

	// all checks passed, we could add the transcation now.
	inserted, oldTx := a.enqueued.Add(tx)
	if !inserted {
		return nil, ErrUnderpriced
	}

	return oldTx, nil
}

// Promote moves eligible transactions from enqueued to promoted.
//
// Eligible transactions are all sequential in order of nonce
// and the first one has to have nonce less (or equal) to the account's
// nextNonce. Lower nonce transaction would be dropped when promoting.
func (a *account) promote() (
	promoted []*types.Transaction,
	dropped []*types.Transaction,
	replaced []*types.Transaction,
) {
	a.promoted.lock(true)
	a.enqueued.lock(true)

	defer func() {
		a.enqueued.unlock()
		a.promoted.unlock()
	}()

	// sanity check
	currentNonce := a.getNonce()
	if a.enqueued.length() == 0 ||
		a.enqueued.peek().Nonce > currentNonce {
		// nothing to promote
		return
	}

	// the first promotable nonce
	nextNonce := currentNonce

	//	move all promotable txs (enqueued txs that are sequential in nonce)
	//	to the account's promoted queue
	for {
		tx := a.enqueued.peek()
		if tx == nil {
			break // no transcation
		}

		// find replacable tx first
		if old := a.promoted.GetTxByNonce(tx.Nonce); old != nil {
			// pop out the transaction first
			tx = a.enqueued.pop()

			if _, old = a.promoted.Add(tx); old != nil {
				// succed to replace old transaction
				replaced = append(replaced, old)
				promoted = append(promoted, tx)
			}

			continue
		}

		if tx.Nonce < nextNonce {
			// pop out too low nonce tx, which should be drop
			tx = a.enqueued.pop()
			dropped = append(dropped, tx)

			continue
		} else if tx.Nonce > nextNonce {
			// nothing to prmote
			break
		}

		// pop from enqueued
		tx = a.enqueued.pop()

		if inserted, _ := a.promoted.Add(tx); !inserted {
			// failed tx would not stop promoting.
			continue
		}

		// update counters
		nextNonce += 1

		// update return result
		promoted = append(promoted, tx)
	}

	// only update the nonce map if the new nonce
	// is higher than the one previously stored.
	if nextNonce > currentNonce {
		a.setNonce(nextNonce)
		// only update the promotion timestamp when it is actually promoted.
		a.updatePromoted()
	}

	return
}

// updatePromoted updates promoted timestamp
func (a *account) updatePromoted() {
	a.lastPromoted = time.Now()
}

// IsOutdated returns whether account was outdated comparing with the outdate time bound,
// the promoted timestamp before the bound is outdated.
func (a *account) IsOutdated(outdateTimeBound time.Time) bool {
	return a.lastPromoted.Before(outdateTimeBound)
}

func txPriceReplacable(newTx, oldTx *types.Transaction) bool {
	return newTx.GasPrice.Cmp(oldTx.GasPrice) > 0
}
