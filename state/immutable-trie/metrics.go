package itrie

import (
	"time"

	"github.com/dogechain-lab/dogechain/helper/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

type MetricsTimeendRecord func()

type Metrics interface {
	// Read code cache hit
	codeCacheHitInc()

	// Read code cache miss
	codeCacheMissInc()

	// Read account cache hit
	accountCacheHitInc()

	// Read account cache miss
	accountCacheMissInc()

	// Read account count from disk
	accountReadCountInc()

	// Number of inserted nodes per transaction
	transactionInsertObserve(int)

	// Number of deleted nodes per transaction
	transactionDeleteObserve(int)

	// Number of new accounts per transaction
	transactionNewAccountObserve(int)

	// Size of written nodes per transaction
	transactionWriteNodeSizeObserve(int)

	// Account hash calculation time
	transactionAccountHashSecondsObserve() MetricsTimeendRecord

	// Root hash calculation time
	transactionRootHashSecondsObserve() MetricsTimeendRecord

	// Time consumed for each read code from disk
	codeDiskReadSecondsObserve() MetricsTimeendRecord

	// Time consumed for each read account from disk
	accountDiskReadSecondsObserve() MetricsTimeendRecord

	// Time consumed for each status transaction commit
	stateCommitSecondsObserve() MetricsTimeendRecord
}

// Metrics represents the itrie metrics
type stateDBMetrics struct {
	trackingIOTimer bool

	codeCacheHit  prometheus.Counter
	codeCacheMiss prometheus.Counter

	accountCacheHit  prometheus.Counter
	accountCacheMiss prometheus.Counter
	accountReadCount prometheus.Counter

	txnInsertCount   prometheus.Histogram
	txnDeleteCount   prometheus.Histogram
	txnNewAccount    prometheus.Histogram
	txnWriteNodeSize prometheus.Histogram

	codeDiskReadSeconds    prometheus.Histogram
	accountDiskReadSeconds prometheus.Histogram

	accountHashSeconds prometheus.Histogram
	rootHashSeconds    prometheus.Histogram

	stateCommitSeconds prometheus.Histogram
}

func (m *stateDBMetrics) codeCacheHitInc() {
	metrics.CounterInc(m.codeCacheHit)
}

func (m *stateDBMetrics) codeCacheMissInc() {
	metrics.CounterInc(m.codeCacheMiss)
}

func (m *stateDBMetrics) accountCacheHitInc() {
	metrics.CounterInc(m.accountCacheHit)
}

func (m *stateDBMetrics) accountCacheMissInc() {
	metrics.CounterInc(m.accountCacheMiss)
}

func (m *stateDBMetrics) accountReadCountInc() {
	metrics.CounterInc(m.accountReadCount)
}

func (m *stateDBMetrics) transactionInsertObserve(count int) {
	metrics.HistogramObserve(m.txnInsertCount, float64(count))
}

func (m *stateDBMetrics) transactionDeleteObserve(count int) {
	metrics.HistogramObserve(m.txnDeleteCount, float64(count))
}

func (m *stateDBMetrics) transactionNewAccountObserve(count int) {
	metrics.HistogramObserve(m.txnNewAccount, float64(count))
}

func (m *stateDBMetrics) transactionWriteNodeSizeObserve(size int) {
	metrics.HistogramObserve(m.txnWriteNodeSize, float64(size))
}

func (m *stateDBMetrics) transactionAccountHashSecondsObserve() MetricsTimeendRecord {
	if !m.trackingIOTimer {
		return func() {}
	}

	begin := time.Now()

	return func() {
		metrics.HistogramObserve(m.accountHashSeconds, time.Since(begin).Seconds())
	}
}

func (m *stateDBMetrics) transactionRootHashSecondsObserve() MetricsTimeendRecord {
	if !m.trackingIOTimer {
		return func() {}
	}

	begin := time.Now()

	return func() {
		metrics.HistogramObserve(m.rootHashSeconds, time.Since(begin).Seconds())
	}
}

func (m *stateDBMetrics) codeDiskReadSecondsObserve() MetricsTimeendRecord {
	if !m.trackingIOTimer {
		return func() {}
	}

	begin := time.Now()

	return func() {
		metrics.HistogramObserve(m.codeDiskReadSeconds, time.Since(begin).Seconds())
	}
}

func (m *stateDBMetrics) accountDiskReadSecondsObserve() MetricsTimeendRecord {
	if !m.trackingIOTimer {
		return func() {}
	}

	begin := time.Now()

	return func() {
		metrics.HistogramObserve(m.accountDiskReadSeconds, time.Since(begin).Seconds())
	}
}

func (m *stateDBMetrics) stateCommitSecondsObserve() MetricsTimeendRecord {
	if !m.trackingIOTimer {
		return func() {}
	}

	begin := time.Now()

	return func() {
		metrics.HistogramObserve(m.stateCommitSeconds, time.Since(begin).Seconds())
	}
}

// GetPrometheusMetrics return the blockchain metrics instance
func GetPrometheusMetrics(namespace string, trackingIOTimer bool, labelsWithValues ...string) Metrics {
	constLabels := metrics.ParseLables(labelsWithValues...)

	m := &stateDBMetrics{
		trackingIOTimer: trackingIOTimer,
		codeCacheHit: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   "itrie",
			Name:        "state_code_cache_hit",
			Help:        "state code cache hit count",
			ConstLabels: constLabels,
		}),
		codeCacheMiss: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   "itrie",
			Name:        "state_code_cache_miss",
			Help:        "state code cache miss count",
			ConstLabels: constLabels,
		}),
		accountCacheHit: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   "itrie",
			Name:        "state_account_cache_hit",
			Help:        "state account cache hit count",
			ConstLabels: constLabels,
		}),
		accountCacheMiss: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   "itrie",
			Name:        "state_account_cache_miss",
			Help:        "state account cache miss count",
			ConstLabels: constLabels,
		}),
		accountReadCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   "itrie",
			Name:        "state_account_read_count",
			Help:        "state account read count",
			ConstLabels: constLabels,
		}),
		txnInsertCount: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace:   namespace,
			Subsystem:   "itrie",
			Name:        "state_transaction_insert_count",
			Help:        "state transaction insert count",
			ConstLabels: constLabels,
		}),
		txnDeleteCount: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace:   namespace,
			Subsystem:   "itrie",
			Name:        "state_transaction_delete_count",
			Help:        "state transaction delete count",
			ConstLabels: constLabels,
		}),
		txnNewAccount: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace:   namespace,
			Subsystem:   "itrie",
			Name:        "state_transaction_new_account_count",
			Help:        "state transaction new account count",
			ConstLabels: constLabels,
		}),
		txnWriteNodeSize: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "itrie",
			Name:      "state_transaction_write_node_size",
			Help:      "state transaction write node size",
		}),
		codeDiskReadSeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace:   namespace,
			Subsystem:   "itrie",
			Name:        "state_code_disk_read_seconds",
			Help:        "state code disk read seconds",
			ConstLabels: constLabels,
		}),
		accountDiskReadSeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace:   namespace,
			Subsystem:   "itrie",
			Name:        "state_account_disk_read_seconds",
			Help:        "state account disk read seconds",
			ConstLabels: constLabels,
		}),
		accountHashSeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace:   namespace,
			Subsystem:   "itrie",
			Name:        "account_hash_seconds",
			Help:        "account hash seconds",
			ConstLabels: constLabels,
		}),
		rootHashSeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace:   namespace,
			Subsystem:   "itrie",
			Name:        "root_hash_seconds",
			Help:        "root hash seconds",
			ConstLabels: constLabels,
		}),
		stateCommitSeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace:   namespace,
			Subsystem:   "itrie",
			Name:        "state_commit_seconds",
			Help:        "state commit seconds",
			ConstLabels: constLabels,
		}),
	}

	prometheus.MustRegister(
		m.codeCacheHit,
		m.codeCacheMiss,
		m.accountCacheHit,
		m.accountCacheMiss,
		m.accountReadCount,
		m.txnInsertCount,
		m.txnDeleteCount,
		m.txnNewAccount,
		m.txnWriteNodeSize,
		m.codeDiskReadSeconds,
		m.accountDiskReadSeconds,
		m.accountHashSeconds,
		m.rootHashSeconds,
		m.stateCommitSeconds,
	)

	return m
}

// NilMetrics will return the non operational blockchain metrics
func NilMetrics() Metrics {
	return &stateDBMetrics{
		trackingIOTimer: false,
	}
}

// NewDummyMetrics will return the no nil blockchain metrics
// TODO: use generic replace this in golang 1.18
func newDummyMetrics(metrics Metrics) Metrics {
	if metrics != nil {
		return metrics
	}

	return NilMetrics()
}
