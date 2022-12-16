package itrie

import (
	"time"

	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/discard"

	prometheus "github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
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
	transactionInsertCount(int)

	// Number of deleted nodes per transaction
	transactionDeleteCount(int)

	// Number of new accounts per transaction
	transactionNewAccountCount(int)

	// Size of written nodes per transaction
	transactionWriteNodeSize(int)

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

	codeCacheHit  metrics.Counter
	codeCacheMiss metrics.Counter

	accountCacheHit  metrics.Counter
	accountCacheMiss metrics.Counter
	accountReadCount metrics.Counter

	txnInsertCount   metrics.Histogram
	txnDeleteCount   metrics.Histogram
	txnNewAccount    metrics.Histogram
	txnWriteNodeSize metrics.Histogram

	codeDiskReadSeconds    metrics.Histogram
	accountDiskReadSeconds metrics.Histogram

	accountHashSeconds metrics.Histogram
	rootHashSeconds    metrics.Histogram

	stateCommitSeconds metrics.Histogram
}

func (m *stateDBMetrics) codeCacheHitInc() {
	m.codeCacheHit.Add(1)
}

func (m *stateDBMetrics) codeCacheMissInc() {
	m.codeCacheMiss.Add(1)
}

func (m *stateDBMetrics) accountCacheHitInc() {
	m.accountCacheHit.Add(1)
}

func (m *stateDBMetrics) accountCacheMissInc() {
	m.accountCacheMiss.Add(1)
}

func (m *stateDBMetrics) accountReadCountInc() {
	m.accountReadCount.Add(1)
}

func (m *stateDBMetrics) transactionInsertCount(count int) {
	m.txnInsertCount.Observe(float64(count))
}

func (m *stateDBMetrics) transactionDeleteCount(count int) {
	m.txnDeleteCount.Observe(float64(count))
}

func (m *stateDBMetrics) transactionNewAccountCount(count int) {
	m.txnNewAccount.Observe(float64(count))
}

func (m *stateDBMetrics) transactionWriteNodeSize(size int) {
	m.txnWriteNodeSize.Observe(float64(size))
}

func (m *stateDBMetrics) transactionAccountHashSecondsObserve() MetricsTimeendRecord {
	if !m.trackingIOTimer {
		return func() {}
	}

	begin := time.Now()

	return func() {
		m.accountHashSeconds.Observe(time.Since(begin).Seconds())
	}
}

func (m *stateDBMetrics) transactionRootHashSecondsObserve() MetricsTimeendRecord {
	if !m.trackingIOTimer {
		return func() {}
	}

	begin := time.Now()

	return func() {
		m.rootHashSeconds.Observe(time.Since(begin).Seconds())
	}
}

func (m *stateDBMetrics) codeDiskReadSecondsObserve() MetricsTimeendRecord {
	if !m.trackingIOTimer {
		return func() {}
	}

	begin := time.Now()

	return func() {
		m.codeDiskReadSeconds.Observe(time.Since(begin).Seconds())
	}
}

func (m *stateDBMetrics) accountDiskReadSecondsObserve() MetricsTimeendRecord {
	if !m.trackingIOTimer {
		return func() {}
	}

	begin := time.Now()

	return func() {
		m.accountDiskReadSeconds.Observe(time.Since(begin).Seconds())
	}
}

func (m *stateDBMetrics) stateCommitSecondsObserve() MetricsTimeendRecord {
	if !m.trackingIOTimer {
		return func() {}
	}

	begin := time.Now()

	return func() {
		m.stateCommitSeconds.Observe(time.Since(begin).Seconds())
	}
}

// GetPrometheusMetrics return the blockchain metrics instance
func GetPrometheusMetrics(namespace string, trackingIOTimer bool, labelsWithValues ...string) Metrics {
	labels := []string{}

	for i := 0; i < len(labelsWithValues); i += 2 {
		labels = append(labels, labelsWithValues[i])
	}

	return &stateDBMetrics{
		trackingIOTimer: trackingIOTimer,
		codeCacheHit: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "itrie",
			Name:      "state_code_cache_hit",
			Help:      "state code cache hit count",
		}, labels).With(labelsWithValues...),
		codeCacheMiss: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "itrie",
			Name:      "state_code_cache_miss",
			Help:      "state code cache miss count",
		}, labels).With(labelsWithValues...),
		accountCacheHit: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "itrie",
			Name:      "state_account_cache_hit",
			Help:      "state account cache hit count",
		}, labels).With(labelsWithValues...),
		accountCacheMiss: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "itrie",
			Name:      "state_account_cache_miss",
			Help:      "state account cache miss count",
		}, labels).With(labelsWithValues...),
		accountReadCount: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "itrie",
			Name:      "state_account_read_count",
			Help:      "state account read count",
		}, labels).With(labelsWithValues...),
		txnInsertCount: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "itrie",
			Name:      "state_transaction_insert_count",
			Help:      "state transaction insert count",
		}, labels).With(labelsWithValues...),
		txnDeleteCount: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "itrie",
			Name:      "state_transaction_delete_count",
			Help:      "state transaction delete count",
		}, labels).With(labelsWithValues...),
		txnNewAccount: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "itrie",
			Name:      "state_transaction_new_account_count",
			Help:      "state transaction new account count",
		}, labels).With(labelsWithValues...),
		txnWriteNodeSize: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "itrie",
			Name:      "state_transaction_write_node_size",
			Help:      "state transaction write node size",
		}, labels).With(labelsWithValues...),
		codeDiskReadSeconds: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "itrie",
			Name:      "state_code_disk_read_seconds",
			Help:      "state code disk read seconds",
		}, labels).With(labelsWithValues...),
		accountDiskReadSeconds: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "itrie",
			Name:      "state_account_disk_read_seconds",
			Help:      "state account disk read seconds",
		}, labels).With(labelsWithValues...),
		accountHashSeconds: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "itrie",
			Name:      "account_hash_seconds",
			Help:      "account hash seconds",
		}, labels).With(labelsWithValues...),
		rootHashSeconds: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "itrie",
			Name:      "root_hash_seconds",
			Help:      "root hash seconds",
		}, labels).With(labelsWithValues...),
		stateCommitSeconds: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "itrie",
			Name:      "state_commit_seconds",
			Help:      "state commit seconds",
		}, labels).With(labelsWithValues...),
	}
}

// NilMetrics will return the non operational blockchain metrics
func NilMetrics() Metrics {
	return &stateDBMetrics{
		trackingIOTimer: false,

		codeCacheHit:     discard.NewCounter(),
		codeCacheMiss:    discard.NewCounter(),
		accountCacheHit:  discard.NewCounter(),
		accountCacheMiss: discard.NewCounter(),
		accountReadCount: discard.NewCounter(),

		txnInsertCount:   discard.NewHistogram(),
		txnDeleteCount:   discard.NewHistogram(),
		txnNewAccount:    discard.NewHistogram(),
		txnWriteNodeSize: discard.NewHistogram(),

		codeDiskReadSeconds:    discard.NewHistogram(),
		accountDiskReadSeconds: discard.NewHistogram(),

		accountHashSeconds: discard.NewHistogram(),
		rootHashSeconds:    discard.NewHistogram(),
		stateCommitSeconds: discard.NewHistogram(),
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
