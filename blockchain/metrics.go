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
	GasPriceAverage metrics.Histogram
	// Gas used
	GasUsed metrics.Histogram
	// Block height
	BlockHeight metrics.Gauge
	// Block written duration
	BlockWrittenSeconds metrics.Histogram
	// Block execution duration
	BlockExecutionSeconds metrics.Histogram
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
		GasPriceAverage: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "blockchain",
			Name:      "gas_avg_price",
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
		BlockWrittenSeconds: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "blockchain",
			Name:      "block_write_seconds",
			Help:      "block write time (seconds)",
		}, labels).With(labelsWithValues...),
		BlockExecutionSeconds: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "blockchain",
			Name:      "block_execution_seconds",
			Help:      "block execution time (seconds)",
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
		GasPriceAverage:       discard.NewHistogram(),
		GasUsed:               discard.NewHistogram(),
		BlockHeight:           discard.NewGauge(),
		BlockWrittenSeconds:   discard.NewHistogram(),
		BlockExecutionSeconds: discard.NewHistogram(),
		TransactionNum:        discard.NewHistogram(),
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
