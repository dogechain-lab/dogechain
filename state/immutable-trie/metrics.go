package itrie

import (
	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/discard"

	prometheus "github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

// Metrics represents the itrie metrics
type Metrics struct {
	CodeLruCacheHit   metrics.Counter
	CodeLruCacheMiss  metrics.Counter
	CodeLruCacheRead  metrics.Counter
	CodeLruCacheWrite metrics.Counter

	AccountStateLruCacheHit metrics.Counter
	TrieStateLruCacheHit    metrics.Counter

	StateLruCacheMiss metrics.Counter
}

// GetPrometheusMetrics return the blockchain metrics instance
func GetPrometheusMetrics(namespace string, labelsWithValues ...string) *Metrics {
	labels := []string{}

	for i := 0; i < len(labelsWithValues); i += 2 {
		labels = append(labels, labelsWithValues[i])
	}

	return &Metrics{
		CodeLruCacheHit: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "itrie",
			Name:      "state_code_lrucache_hit",
			Help:      "state code cache hit count",
		}, labels).With(labelsWithValues...),
		CodeLruCacheMiss: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "itrie",
			Name:      "state_code_lrucache_miss",
			Help:      "state code cache miss count",
		}, labels).With(labelsWithValues...),
		CodeLruCacheRead: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "itrie",
			Name:      "state_code_lrucache_read",
			Help:      "state code cache read count",
		}, labels).With(labelsWithValues...),
		CodeLruCacheWrite: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "itrie",
			Name:      "state_code_lrucache_write",
			Help:      "state code cache write count",
		}, labels).With(labelsWithValues...),
		AccountStateLruCacheHit: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "itrie",
			Name:      "account_state_snapshot_lrucache_hit",
			Help:      "account state snapshot cache hit count",
		}, labels).With(labelsWithValues...),
		TrieStateLruCacheHit: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "itrie",
			Name:      "trie_state_snapshot_lrucache_hit",
			Help:      "trie state snapshot cache hit count",
		}, labels).With(labelsWithValues...),
		StateLruCacheMiss: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "itrie",
			Name:      "state_snapshot_lrucache_miss",
			Help:      "trie state snapshot cache miss count",
		}, labels).With(labelsWithValues...),
	}
}

// NilMetrics will return the non operational blockchain metrics
func NilMetrics() *Metrics {
	return &Metrics{
		CodeLruCacheHit:   discard.NewCounter(),
		CodeLruCacheMiss:  discard.NewCounter(),
		CodeLruCacheRead:  discard.NewCounter(),
		CodeLruCacheWrite: discard.NewCounter(),

		AccountStateLruCacheHit: discard.NewCounter(),
		TrieStateLruCacheHit:    discard.NewCounter(),

		StateLruCacheMiss: discard.NewCounter(),
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
