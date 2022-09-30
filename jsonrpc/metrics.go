package jsonrpc

import (
	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/discard"
	prometheus "github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

// Metrics represents the jsonrpc metrics
type Metrics struct {
	// Requests number
	Requests metrics.Counter

	// Errors number
	Errors metrics.Counter

	// Requests duration (seconds)
	RequestDurationSeconds metrics.Histogram
}

// GetPrometheusMetrics return the blockchain metrics instance
func GetPrometheusMetrics(namespace string, labelsWithValues ...string) *Metrics {
	labels := []string{}

	for i := 0; i < len(labelsWithValues); i += 2 {
		labels = append(labels, labelsWithValues[i])
	}

	return &Metrics{
		Requests: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "jsonrpc",
			Name:      "requests",
			Help:      "Requests number",
		}, labels).With(labelsWithValues...),
		Errors: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "jsonrpc",
			Name:      "request_errors",
			Help:      "Request errors number",
		}, labels).With(labelsWithValues...),
		RequestDurationSeconds: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "jsonrpc",
			Name:      "request_duration_seconds",
			Help:      "Request duration (seconds)",
			Buckets: []float64{
				0.01,
				0.5,
				0.99,
				1.0,
			},
		}, labels).With(labelsWithValues...),
	}
}

// NilMetrics will return the non operational blockchain metrics
func NilMetrics() *Metrics {
	return &Metrics{
		Requests:               discard.NewCounter(),
		Errors:                 discard.NewCounter(),
		RequestDurationSeconds: discard.NewHistogram(),
	}
}
