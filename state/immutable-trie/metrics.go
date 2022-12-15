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
	CodeCacheHitInc()
	CodeCacheMissInc()

	AccountCacheHitInc()
	AccountCacheMissInc()

	CodeDiskReadSecondsObserve() MetricsTimeendRecord
	AccountDiskReadSecondsObserve() MetricsTimeendRecord

	StateCommitSecondsObserve() MetricsTimeendRecord
}

// Metrics represents the itrie metrics
type stateDBMetrics struct {
	trackingIOTimer bool

	codeCacheHit  metrics.Counter
	codeCacheMiss metrics.Counter

	accountCacheHit  metrics.Counter
	accountCacheMiss metrics.Counter

	codeDiskReadSeconds    metrics.Histogram
	accountDiskReadSeconds metrics.Histogram

	stateCommitSeconds metrics.Histogram
}

func (m *stateDBMetrics) CodeCacheHitInc() {
	m.codeCacheHit.Add(1)
}

func (m *stateDBMetrics) CodeCacheMissInc() {
	m.codeCacheMiss.Add(1)
}

func (m *stateDBMetrics) AccountCacheHitInc() {
	m.accountCacheHit.Add(1)
}

func (m *stateDBMetrics) AccountCacheMissInc() {
	m.accountCacheMiss.Add(1)
}

func (m *stateDBMetrics) CodeDiskReadSecondsObserve() MetricsTimeendRecord {
	if !m.trackingIOTimer {
		return func() {}
	}

	begin := time.Now()

	return func() {
		m.codeDiskReadSeconds.Observe(time.Since(begin).Seconds())
	}
}

func (m *stateDBMetrics) AccountDiskReadSecondsObserve() MetricsTimeendRecord {
	if !m.trackingIOTimer {
		return func() {}
	}

	begin := time.Now()

	return func() {
		m.accountDiskReadSeconds.Observe(time.Since(begin).Seconds())
	}
}

func (m *stateDBMetrics) StateCommitSecondsObserve() MetricsTimeendRecord {
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

		codeDiskReadSeconds:    discard.NewHistogram(),
		accountDiskReadSeconds: discard.NewHistogram(),

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
