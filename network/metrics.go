package network

import (
	"github.com/dogechain-lab/dogechain/helper/metrics"
	"github.com/dogechain-lab/dogechain/network/client"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics represents the network metrics
type Metrics struct {
	// Number of connected peers
	totalPeerCount prometheus.Gauge

	// Number of outbound connections
	outboundConnectionsCount prometheus.Gauge

	// Number of inbound connections
	inboundConnectionsCount prometheus.Gauge

	// Number of pending outbound connections
	pendingOutboundConnectionsCount prometheus.Gauge

	// Number of pending inbound connections
	pendingInboundConnectionsCount prometheus.Gauge

	// Create new proto connection duration
	newProtoConnectionSecond prometheus.Histogram

	// Create new proto connection count
	newProtoConnectionCount prometheus.Counter

	// Create new proto connection error count
	newProtoConnectionErrorCount prometheus.Counter

	// Grpc client metrics
	grpcMetrics client.Metrics
}

func (m *Metrics) SetTotalPeerCount(v float64) {
	metrics.SetGauge(m.totalPeerCount, v)
}

func (m *Metrics) SetOutboundConnectionsCount(v float64) {
	metrics.SetGauge(m.outboundConnectionsCount, v)
}

func (m *Metrics) SetInboundConnectionsCount(v float64) {
	metrics.SetGauge(m.inboundConnectionsCount, v)
}

func (m *Metrics) SetPendingOutboundConnectionsCount(v float64) {
	metrics.SetGauge(m.pendingOutboundConnectionsCount, v)
}

func (m *Metrics) SetPendingInboundConnectionsCount(v float64) {
	metrics.SetGauge(m.pendingInboundConnectionsCount, v)
}

func (m *Metrics) NewProtoConnectionSecondObserve(v float64) {
	metrics.HistogramObserve(m.newProtoConnectionSecond, v)
}

func (m *Metrics) NewProtoConnectionCountInc() {
	metrics.CounterInc(m.newProtoConnectionCount)
}

func (m *Metrics) NewProtoConnectionErrorCountInc() {
	metrics.CounterInc(m.newProtoConnectionErrorCount)
}

func (m *Metrics) GetGrpcMetrics() client.Metrics {
	return m.grpcMetrics
}

// GetPrometheusMetrics return the network metrics instance
func GetPrometheusMetrics(namespace string, labelsWithValues ...string) *Metrics {
	constLabels := metrics.ParseLables(labelsWithValues...)

	m := &Metrics{
		totalPeerCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   "network",
			Name:        "peers",
			Help:        "Number of connected peers",
			ConstLabels: constLabels,
		}),
		outboundConnectionsCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   "network",
			Name:        "outbound_connections_count",
			Help:        "Number of outbound connections",
			ConstLabels: constLabels,
		}),
		inboundConnectionsCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   "network",
			Name:        "inbound_connections_count",
			Help:        "Number of inbound connections",
			ConstLabels: constLabels,
		}),
		pendingOutboundConnectionsCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   "network",
			Name:        "pending_outbound_connections_count",
			Help:        "Number of pending outbound connections",
			ConstLabels: constLabels,
		}),
		pendingInboundConnectionsCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   "network",
			Name:        "pending_inbound_connections_count",
			Help:        "Number of pending inbound connections",
			ConstLabels: constLabels,
		}),
		newProtoConnectionSecond: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace:   namespace,
			Subsystem:   "network",
			Name:        "new_proto_connection_second",
			Help:        "create new proto connection duration",
			ConstLabels: constLabels,
		}),
		newProtoConnectionCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   "network",
			Name:        "new_proto_connection_count",
			Help:        "create new proto connection count",
			ConstLabels: constLabels,
		}),
		newProtoConnectionErrorCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   "network",
			Name:        "new_proto_connection_error_count",
			Help:        "create new proto connection error count",
			ConstLabels: constLabels,
		}),
		grpcMetrics: client.NewMetrics(),
	}

	prometheus.MustRegister(
		m.totalPeerCount,
		m.outboundConnectionsCount,
		m.inboundConnectionsCount,
		m.pendingOutboundConnectionsCount,
		m.pendingInboundConnectionsCount,
		m.newProtoConnectionSecond,
		m.newProtoConnectionCount,
		m.newProtoConnectionErrorCount,
	)

	return m
}

// NilMetrics will return the non-operational metrics
func NilMetrics() *Metrics {
	return &Metrics{
		grpcMetrics: client.NilMetrics(),
	}
}
