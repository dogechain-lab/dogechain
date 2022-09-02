package txpool

const (
	DefaultPruneTickSeconds      = 300  // ticker duration for pruning account future transactions
	DefaultPromoteOutdateSeconds = 3600 // not promoted account for a long time would be pruned
	// txpool transaction max slots. tx <= 32kB would only take 1 slot. tx > 32kB would take
	// ceil(tx.size / 32kB) slots.
	DefaultMaxSlots                = 4096
	DefaultMaxAccountDemotions     = 10  // account demotion counter limit
	DefaultClippingMemoryThreshold = 80  // when it reach 80%, it will trigger the clipping
	MaxClippingMemoryThreshold     = 100 // maximum is 100%
	DefaultClippingTickSeconds     = 60  // clipping check duration, in seconds.
)
