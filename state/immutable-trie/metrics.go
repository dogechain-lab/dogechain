package itrie

import (
	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/discard"

	prometheus "github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

// Metrics represents the itrie metrics
type Metrics struct {
	MemCacheHit   metrics.Counter
	MemCacheMiss  metrics.Counter
	MemCacheRead  metrics.Counter
	MemCacheWrite metrics.Counter

	AccountStateLruCacheHit  metrics.Counter
	AccountStateLruCacheMiss metrics.Counter

	TrieStateLruCacheHit  metrics.Counter
	TrieStateLruCacheMiss metrics.Counter
}

// GetPrometheusMetrics return the blockchain metrics instance
func GetPrometheusMetrics(namespace string, labelsWithValues ...string) *Metrics {
	labels := []string{}

	for i := 0; i < len(labelsWithValues); i += 2 {
		labels = append(labels, labelsWithValues[i])
	}

	return &Metrics{
		MemCacheHit: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "itrie",
			Name:      "state_code_memcache_hit",
			Help:      "state code cache hit count",
		}, labels).With(labelsWithValues...),
		MemCacheMiss: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "itrie",
			Name:      "state_code_memcache_miss",
			Help:      "state code cache miss count",
		}, labels).With(labelsWithValues...),
		MemCacheRead: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "itrie",
			Name:      "state_code_memcache_read",
			Help:      "state code cache read count",
		}, labels).With(labelsWithValues...),
		MemCacheWrite: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "itrie",
			Name:      "state_code_memcache_write",
			Help:      "state code cache write count",
		}, labels).With(labelsWithValues...),
		AccountStateLruCacheHit: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "itrie",
			Name:      "account_state_snapshot_lrucache_hit",
			Help:      "account state snapshot cache hit count",
		}, labels).With(labelsWithValues...),
		AccountStateLruCacheMiss: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "itrie",
			Name:      "account_state_snapshot_lrucache_miss",
			Help:      "account state snapshot cache miss count",
		}, labels).With(labelsWithValues...),
		TrieStateLruCacheHit: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "itrie",
			Name:      "trie_state_snapshot_lrucache_hit",
			Help:      "trie state snapshot cache hit count",
		}, labels).With(labelsWithValues...),
		TrieStateLruCacheMiss: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "itrie",
			Name:      "trie_state_snapshot_lrucache_miss",
			Help:      "trie state snapshot cache miss count",
		}, labels).With(labelsWithValues...),
	}
}

// NilMetrics will return the non operational blockchain metrics
func NilMetrics() *Metrics {
	return &Metrics{
		MemCacheHit:   discard.NewCounter(),
		MemCacheMiss:  discard.NewCounter(),
		MemCacheRead:  discard.NewCounter(),
		MemCacheWrite: discard.NewCounter(),

		AccountStateLruCacheHit:  discard.NewCounter(),
		AccountStateLruCacheMiss: discard.NewCounter(),

		TrieStateLruCacheHit:  discard.NewCounter(),
		TrieStateLruCacheMiss: discard.NewCounter(),
	}
}

// NewDummyMetrics will return the no nil blockchain metrics
// TODO: use generic replace this in golang 1.18
func NewDummyMetrics(metrics *Metrics) *Metrics {
	if metrics != nil {
		return metrics
	}

	return NilMetrics()
}
