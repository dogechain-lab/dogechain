package consensus

import (
	"github.com/dogechain-lab/dogechain/helper/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics represents the consensus metrics
type Metrics struct {
	// No.of validators
	validators prometheus.Gauge
	// No.of rounds
	rounds prometheus.Gauge
	// No.of transactions in the block
	numTxs prometheus.Gauge
	//Time between current block and the previous block in seconds
	blockInterval prometheus.Gauge
}

func (m *Metrics) SetValidators(val float64) {
	metrics.SetGauge(m.validators, val)
}

func (m *Metrics) SetRounds(val float64) {
	metrics.SetGauge(m.rounds, val)
}

func (m *Metrics) SetNumTxs(val float64) {
	metrics.SetGauge(m.numTxs, val)
}

func (m *Metrics) SetBlockInterval(val float64) {
	metrics.SetGauge(m.blockInterval, val)
}

// GetPrometheusMetrics return the consensus metrics instance
func GetPrometheusMetrics(namespace string, labelsWithValues ...string) *Metrics {
	constLabels := metrics.ParseLables(labelsWithValues...)

	return &Metrics{
		validators: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   "consensus",
			Name:        "validators",
			Help:        "Number of validators.",
			ConstLabels: constLabels,
		}),
		rounds: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   "consensus",
			Name:        "rounds",
			Help:        "Number of rounds.",
			ConstLabels: constLabels,
		}),
		numTxs: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   "consensus",
			Name:        "num_txs",
			Help:        "Number of transactions.",
			ConstLabels: constLabels,
		}),
		blockInterval: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   "consensus",
			Name:        "block_interval",
			Help:        "Time between current block and the previous block in seconds.",
			ConstLabels: constLabels,
		}),
	}
}

// NilMetrics will return the non operational metrics
func NilMetrics() *Metrics {
	return &Metrics{}
}
