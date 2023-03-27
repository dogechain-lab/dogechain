package blockchain

import (
	"github.com/dogechain-lab/dogechain/helper/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

const subsystem = "blockchain"

// Metrics represents the blockchain metrics
type Metrics struct {
	// Max gas price
	maxGasPrice prometheus.Histogram
	// Gas Price Average
	gasPriceAverage prometheus.Histogram
	// Gas used
	gasUsed prometheus.Histogram
	// Block height
	blockHeight prometheus.Gauge
	// Block written duration
	blockWrittenSeconds prometheus.Histogram
	// Block execution duration
	blockExecutionSeconds prometheus.Histogram
	// Transaction number
	transactionNum prometheus.Histogram
}

func (m *Metrics) MaxGasPriceObserve(v float64) {
	metrics.HistogramObserve(m.maxGasPrice, v)
}

func (m *Metrics) GasPriceAverageObserve(v float64) {
	metrics.HistogramObserve(m.gasPriceAverage, v)
}

func (m *Metrics) GasUsedObserve(v float64) {
	metrics.HistogramObserve(m.gasUsed, v)
}

func (m *Metrics) SetBlockHeight(v float64) {
	metrics.SetGauge(m.blockHeight, v)
}

func (m *Metrics) BlockWrittenSecondsObserve(v float64) {
	metrics.HistogramObserve(m.blockWrittenSeconds, v)
}

func (m *Metrics) BlockExecutionSecondsObserve(v float64) {
	metrics.HistogramObserve(m.blockExecutionSeconds, v)
}

func (m *Metrics) TransactionNumObserve(v float64) {
	metrics.HistogramObserve(m.transactionNum, v)
}

// GetPrometheusMetrics return the blockchain metrics instance
func GetPrometheusMetrics(namespace string, labelsWithValues ...string) *Metrics {
	constLabels := metrics.ParseLables(labelsWithValues...)

	m := &Metrics{
		maxGasPrice: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "max_gas_price",
			Help:        "max gas price within the block exclude miner transactions",
			ConstLabels: constLabels,
		}),
		gasPriceAverage: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "gas_avg_price",
			Help:        "avg gas price within the block exclude miner transactions",
			ConstLabels: constLabels,
		}),
		gasUsed: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "gas_used",
			Help:        "total gas used within the block",
			ConstLabels: constLabels,
		}),
		blockHeight: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "block_height",
			Help:        "current block height",
			ConstLabels: constLabels,
		}),
		blockWrittenSeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "block_write_seconds",
			Help:        "block write time (seconds)",
			ConstLabels: constLabels,
		}),
		blockExecutionSeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "block_execution_seconds",
			Help:        "block execution time (seconds)",
			ConstLabels: constLabels,
		}),
		transactionNum: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "transaction_number",
			Help:        "Transaction number",
			ConstLabels: constLabels,
		}),
	}

	prometheus.MustRegister(
		m.maxGasPrice,
		m.gasPriceAverage,
		m.gasUsed,
		m.blockHeight,
		m.blockWrittenSeconds,
		m.blockExecutionSeconds,
		m.transactionNum,
	)

	return m
}

// NilMetrics will return the non operational blockchain metrics
func NilMetrics() *Metrics {
	return &Metrics{}
}

// NewDummyMetrics will return the no nil blockchain metrics
// TODO: use generic replace this in golang 1.18
func NewDummyMetrics(metrics *Metrics) *Metrics {
	if metrics != nil {
		return metrics
	}

	return NilMetrics()
}
