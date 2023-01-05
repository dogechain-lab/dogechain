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

	// Eth metrics
	ethAPI *stdprometheus.CounterVec

	// Net metrics
	netAPI *stdprometheus.CounterVec

	// Web3 metrics
	web3API *stdprometheus.CounterVec

	// TxPool metrics
	txPoolAPI *stdprometheus.CounterVec

	// Debug metrics
	debugAPI *stdprometheus.CounterVec
}

func (m *Metrics) EthAPICounterInc(method string) {
	if m.ethAPI != nil {
		m.ethAPI.With(stdprometheus.Labels{"method": "eth_" + method}).Inc()
	}
}

func (m *Metrics) NetAPICounterInc(method string) {
	if m.netAPI != nil {
		m.netAPI.With(stdprometheus.Labels{"method": "net_" + method}).Inc()
	}
}

func (m *Metrics) Web3APICounterInc(method string) {
	if m.web3API != nil {
		m.web3API.With(stdprometheus.Labels{"method": "web3_" + method}).Inc()
	}
}

func (m *Metrics) TxPoolAPICounterInc(method string) {
	if m.txPoolAPI != nil {
		m.txPoolAPI.With(stdprometheus.Labels{"method": "txpool_" + method}).Inc()
	}
}

func (m *Metrics) DebugAPICounterInc(method string) {
	if m.debugAPI != nil {
		m.debugAPI.With(stdprometheus.Labels{"method": "debug_" + method}).Inc()
	}
}

// GetPrometheusMetrics return the blockchain metrics instance
func GetPrometheusMetrics(namespace string, labelsWithValues ...string) *Metrics {
	labels := []string{}

	for i := 0; i < len(labelsWithValues); i += 2 {
		labels = append(labels, labelsWithValues[i])
	}

	constLabels := map[string]string{}
	for i := 1; i < len(labelsWithValues); i += 2 {
		constLabels[labelsWithValues[i-1]] = labelsWithValues[i]
	}

	m := &Metrics{
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
		ethAPI: stdprometheus.NewCounterVec(stdprometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   "jsonrpc",
			Name:        "eth_api_requests",
			Help:        "eth api requests",
			ConstLabels: constLabels,
		}, []string{"method"}),
		netAPI: stdprometheus.NewCounterVec(stdprometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   "jsonrpc",
			Name:        "net_api_requests",
			Help:        "net api requests",
			ConstLabels: constLabels,
		}, []string{"method"}),
		web3API: stdprometheus.NewCounterVec(stdprometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   "jsonrpc",
			Name:        "web3_api_requests",
			Help:        "web3 api requests",
			ConstLabels: constLabels,
		}, []string{"method"}),
		txPoolAPI: stdprometheus.NewCounterVec(stdprometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   "jsonrpc",
			Name:        "txpool_api_requests",
			Help:        "TxPool api requests",
			ConstLabels: constLabels,
		}, []string{"method"}),
		debugAPI: stdprometheus.NewCounterVec(stdprometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   "jsonrpc",
			Name:        "debug_api_requests",
			Help:        "debug api requests",
			ConstLabels: constLabels,
		}, []string{"method"}),
	}

	stdprometheus.MustRegister(
		m.ethAPI,
		m.netAPI,
		m.web3API,
		m.txPoolAPI,
		m.debugAPI,
	)

	return m
}

// NilMetrics will return the non operational jsonrpc metrics
func NilMetrics() *Metrics {
	return &Metrics{
		Requests:     discard.NewCounter(),
		Errors:       discard.NewCounter(),
		ResponseTime: discard.NewHistogram(),
		ethAPI:       nil,
		netAPI:       nil,
		web3API:      nil,
		txPoolAPI:    nil,
		debugAPI:     nil,
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
