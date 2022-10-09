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
	ResponseTime metrics.Histogram
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
		ResponseTime: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "jsonrpc",
			Name:      "response_seconds",
			Help:      "Response time (seconds)",
			Buckets: []float64{
				0.001,
				0.01,
				0.1,
				0.5,
				1.0,
				2.0,
			},
		}, labels).With(labelsWithValues...),
	}
}

// NilMetrics will return the non operational jsonrpc metrics
func NilMetrics() *Metrics {
	return &Metrics{
		Requests:     discard.NewCounter(),
		Errors:       discard.NewCounter(),
		ResponseTime: discard.NewHistogram(),
	}
}

// NewDummyMetrics will return the no nil jsonrpc metrics
// TODO: use generic replace this in golang 1.18
func NewDummyMetrics(metrics *Metrics) *Metrics {
	if metrics != nil {
		return metrics
	}

	return NilMetrics()
}
