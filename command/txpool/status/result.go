package status

import (
	"bytes"
	"fmt"

	"github.com/dogechain-lab/dogechain/command/helper"
)

type TxPoolStatusResult struct {
	Transactions         uint64 `json:"transactions"`
	PendingTransactions  uint64 `json:"pendingTransactions"`
	EnqueuedTransactions uint64 `json:"enqueuedTransactions"`
	MaxSlots             uint64 `json:"maxSlots"`
}

func (r *TxPoolStatusResult) GetOutput() string {
	var buffer bytes.Buffer

	buffer.WriteString("\n[TXPOOL STATUS]\n")
	buffer.WriteString(helper.FormatKV([]string{
		fmt.Sprintf("Pending transactions|%d", r.PendingTransactions),
		fmt.Sprintf("Enqueued transactions|%d", r.EnqueuedTransactions),
		fmt.Sprintf("Max slots|%d", r.MaxSlots),
	}))
	buffer.WriteString("\n")

	return buffer.String()
}
