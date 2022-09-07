package txpool

const (
	DefaultPruneTickSeconds      = 300  // ticker duration for pruning account future transactions
	DefaultPromoteOutdateSeconds = 3600 // not promoted account for a long time would be pruned
	// txpool transaction max slots. tx <= 32kB would only take 1 slot. tx > 32kB would take
	// ceil(tx.size / 32kB) slots.
	DefaultMaxSlots = 4096
)
