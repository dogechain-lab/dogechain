package blockchain

import (
	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/discard"
	prometheus "github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

// Metrics represents the blockchain metrics
type Metrics struct {
	// Gas Price Average
	GasPriceAvg metrics.Histogram
	// Gas used
	GasUsed metrics.Histogram
	// Block height
	BlockHeight metrics.Gauge
	// Block write duration time
	BlockWriteDuration metrics.Histogram
	// Transaction number
	TransactionNum metrics.Histogram
}

// GetPrometheusMetrics return the blockchain metrics instance
func GetPrometheusMetrics(namespace string, labelsWithValues ...string) *Metrics {
	labels := []string{}

	for i := 0; i < len(labelsWithValues); i += 2 {
		labels = append(labels, labelsWithValues[i])
	}

	return &Metrics{
		GasPriceAvg: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "blockchain",
			Name:      "gas_price_avg",
			Help:      "Gas Price Average",
		}, labels).With(labelsWithValues...),
		GasUsed: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "blockchain",
			Name:      "gas_used",
			Help:      "Gas Used",
		}, labels).With(labelsWithValues...),
		BlockHeight: prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "blockchain",
			Name:      "block_height",
			Help:      "Block height",
		}, labels).With(labelsWithValues...),
		BlockWriteDuration: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "blockchain",
			Name:      "block_write_duration_seconds",
			Help:      "block write duration (seconds)",
			Buckets: []float64{
				0.01,
				0.5,
				0.99,
				1.0,
			},
		}, labels).With(labelsWithValues...),
		TransactionNum: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "blockchain",
			Name:      "transaction_number",
			Help:      "Transaction number",
		}, labels).With(labelsWithValues...),
	}
}

// NilMetrics will return the non operational blockchain metrics
func NilMetrics() *Metrics {
	return &Metrics{
		GasPriceAvg:        discard.NewHistogram(),
		GasUsed:            discard.NewHistogram(),
		BlockHeight:        discard.NewGauge(),
		BlockWriteDuration: discard.NewHistogram(),
		TransactionNum:     discard.NewHistogram(),
	}
}
