package txpool

import (
	"github.com/dogechain-lab/dogechain/helper/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics represents the txpool metrics
type Metrics struct {
	// Pending transactions
	pendingTxs prometheus.Gauge
	// Enqueue transactions
	enqueueTxs prometheus.Gauge
}

func (m *Metrics) Register() {
	if m.pendingTxs != nil {
		prometheus.MustRegister(m.pendingTxs)
	}

	if m.enqueueTxs != nil {
		prometheus.MustRegister(m.enqueueTxs)
	}
}

func (m *Metrics) AddPendingTxs(v float64) {
	if m.pendingTxs == nil {
		return
	}

	m.pendingTxs.Add(v)
}

func (m *Metrics) SetPendingTxs(v float64) {
	if m.pendingTxs == nil {
		return
	}

	m.pendingTxs.Set(v)
}

func (m *Metrics) AddEnqueueTxs(v float64) {
	if m.enqueueTxs == nil {
		return
	}

	m.enqueueTxs.Add(v)
}

func (m *Metrics) SetEnqueueTxs(v float64) {
	if m.enqueueTxs == nil {
		return
	}

	m.enqueueTxs.Set(v)
}

// GetPrometheusMetrics return the txpool metrics instance
func GetPrometheusMetrics(namespace string, labelsWithValues ...string) *Metrics {
	constLabels := metrics.ParseLables(labelsWithValues...)

	m := &Metrics{
		pendingTxs: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   "txpool",
			Name:        "pending_transactions",
			Help:        "Pending transactions in the pool",
			ConstLabels: constLabels,
		}),
		enqueueTxs: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   "txpool",
			Name:        "enqueued_transactions",
			Help:        "Enqueued transactions in the pool",
			ConstLabels: constLabels,
		}),
	}

	return m
}

// NilMetrics will return the non operational txpool metrics
func NilMetrics() *Metrics {
	return &Metrics{}
}
