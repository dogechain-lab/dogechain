package txpool

import (
	"container/heap"
	"sync"
	"sync/atomic"

	"github.com/dogechain-lab/dogechain/types"
)

// A thread-safe wrapper of a minNonceQueue.
// All methods assume the (correct) lock is held.
type accountQueue struct {
	sync.RWMutex
	wLock uint32
	queue minNonceQueue
	txs   sync.Map // nonce filter transactions
}

func newAccountQueue() *accountQueue {
	q := accountQueue{
		queue: make(minNonceQueue, 0),
	}

	heap.Init(&q.queue)

	return &q
}

func (q *accountQueue) lock(write bool) {
	switch write {
	case true:
		q.Lock()
		atomic.StoreUint32(&q.wLock, 1)
	case false:
		q.RLock()
		atomic.StoreUint32(&q.wLock, 0)
	}
}

func (q *accountQueue) unlock() {
	if atomic.SwapUint32(&q.wLock, 0) == 1 {
		q.Unlock()
	} else {
		q.RUnlock()
	}
}

// prune removes all transactions from the queue
// with nonce lower than given.
func (q *accountQueue) prune(nonce uint64) (
	pruned []*types.Transaction,
) {
	for {
		tx := q.peek()
		if tx == nil ||
			tx.Nonce >= nonce {
			break
		}

		tx = q.pop()
		pruned = append(pruned, tx)
	}

	return
}

// clear removes all transactions from the queue.
func (q *accountQueue) clear() (removed []*types.Transaction) {
	// store txs
	removed = q.queue

	// clear the underlying queue
	q.queue = q.queue[:0]

	// clear the underlying map
	q.clearNonceTxs()

	return
}

// GetTxByNonce returns the specific nonce transaction.
//
// thread-safe
func (q *accountQueue) GetTxByNonce(nonce uint64) *types.Transaction {
	v, ok := q.txs.Load(nonce)
	if !ok {
		return nil
	}

	//nolint:forcetypeassert
	return v.(*types.Transaction)
}

func (q *accountQueue) setNonceTx(tx *types.Transaction) {
	q.txs.Store(tx.Nonce, tx)
}

func (q *accountQueue) deleteNonceTx(nonce uint64) {
	q.txs.Delete(nonce)
}

func (q *accountQueue) clearNonceTxs() {
	q.txs.Range(func(key, value interface{}) bool {
		q.txs.Delete(key)

		return true
	})
}

// Add tries to insert a new transaction into the list, returning whether the
// transaction was accepted, and if yes, any previous transaction it replaced.
//
// not thread-safe, should be lock held.
func (q *accountQueue) SameNonceTx(tx *types.Transaction) (replacable bool, old *types.Transaction) {
	old = q.GetTxByNonce(tx.Nonce)
	if old == nil {
		return false, nil
	}
	// If there's an older better transaction, abort
	if !txPriceReplacable(tx, old) {
		return false, old
	}

	return true, old
}

// Add tries to insert or replace a new transaction into the list, returning
// whether the transaction was accepted, and if yes, any previous transaction
// it replaced.
//
// not thread-safe, should be lock held.
func (q *accountQueue) Add(tx *types.Transaction) (bool, *types.Transaction) {
	replacable, old := q.SameNonceTx(tx)
	if !replacable && old != nil {
		// transaction replace underprice
		return false, old
	}

	// cache nonce
	q.setNonceTx(tx)

	// upsert
	if old == nil {
		q.push(tx)
	} else {
		old = q.replaceTxByNewTx(tx)
	}

	return true, old
}

func (q *accountQueue) replaceTxByNewTx(newTx *types.Transaction) *types.Transaction {
	var dropped *types.Transaction

	for i, tx := range q.queue {
		if tx.Nonce == newTx.Nonce && txPriceReplacable(newTx, tx) {
			dropped = tx
			q.queue[i] = newTx

			break
		}
	}

	return dropped
}

// push pushes the given transaction onto the queue.
func (q *accountQueue) push(tx *types.Transaction) {
	heap.Push(&q.queue, tx)
}

// peek returns the first transaction from the queue without removing it.
func (q *accountQueue) peek() *types.Transaction {
	if q.length() == 0 {
		return nil
	}

	return q.queue.Peek()
}

// pop removes the first transactions from the queue and returns it.
func (q *accountQueue) pop() *types.Transaction {
	if q.length() == 0 {
		return nil
	}

	transaction, ok := heap.Pop(&q.queue).(*types.Transaction)
	if !ok {
		return nil
	}

	// remove it from cache
	q.deleteNonceTx(transaction.Nonce)

	return transaction
}

// length returns the number of transactions in the queue.
func (q *accountQueue) length() uint64 {
	return uint64(q.queue.Len())
}

// transactions sorted by nonce (ascending)
type minNonceQueue []*types.Transaction

/* Queue methods required by the heap interface */

func (q *minNonceQueue) Peek() *types.Transaction {
	if q.Len() == 0 {
		return nil
	}

	return (*q)[0]
}

func (q *minNonceQueue) Len() int {
	return len(*q)
}

func (q *minNonceQueue) Swap(i, j int) {
	(*q)[i], (*q)[j] = (*q)[j], (*q)[i]
}

func (q *minNonceQueue) Less(i, j int) bool {
	return (*q)[i].Nonce < (*q)[j].Nonce
}

func (q *minNonceQueue) Push(x interface{}) {
	transaction, ok := x.(*types.Transaction)
	if !ok {
		return
	}

	*q = append(*q, transaction)
}

func (q *minNonceQueue) Pop() interface{} {
	old := q
	n := len(*old)
	x := (*old)[n-1]
	*q = (*old)[0 : n-1]

	return x
}

type pricedQueue struct {
	queue maxPriceQueue
}

func newPricedQueue() *pricedQueue {
	q := pricedQueue{
		queue: make(maxPriceQueue, 0),
	}

	heap.Init(&q.queue)

	return &q
}

// clear empties the underlying queue.
func (q *pricedQueue) clear() {
	q.queue = q.queue[:0]
}

// Pushes the given transactions onto the queue.
func (q *pricedQueue) push(tx *types.Transaction) {
	heap.Push(&q.queue, tx)
}

// Pop removes the first transaction from the queue
// or nil if the queue is empty.
func (q *pricedQueue) pop() *types.Transaction {
	if q.length() == 0 {
		return nil
	}

	transaction, ok := heap.Pop(&q.queue).(*types.Transaction)
	if !ok {
		return nil
	}

	return transaction
}

// length returns the number of transactions in the queue.
func (q *pricedQueue) length() uint64 {
	return uint64(q.queue.Len())
}

// transactions sorted by gas price (descending)
type maxPriceQueue []*types.Transaction

/* Queue methods required by the heap interface */

func (q *maxPriceQueue) Peek() *types.Transaction {
	if q.Len() == 0 {
		return nil
	}

	return (*q)[0]
}

func (q *maxPriceQueue) Len() int {
	return len(*q)
}

func (q *maxPriceQueue) Swap(i, j int) {
	(*q)[i], (*q)[j] = (*q)[j], (*q)[i]
}

func (q *maxPriceQueue) Less(i, j int) bool {
	return (*q)[i].GasPrice.Uint64() > (*q)[j].GasPrice.Uint64()
}

func (q *maxPriceQueue) Push(x interface{}) {
	transaction, ok := x.(*types.Transaction)
	if !ok {
		return
	}

	*q = append(*q, transaction)
}

func (q *maxPriceQueue) Pop() interface{} {
	old := q
	n := len(*old)
	x := (*old)[n-1]
	*q = (*old)[0 : n-1]

	return x
}
