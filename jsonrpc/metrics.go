package jsonrpc

import (
	"github.com/dogechain-lab/dogechain/helper/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

type EthAPILabels prometheus.Labels

var (
	EthBlockNumberLabel      = EthAPILabels{"method": "eth_blockNumber"}
	EthCallLabel             = EthAPILabels{"method": "eth_call"}
	EthChainIDLabel          = EthAPILabels{"method": "eth_chainId"}
	EthEstimateGasLabel      = EthAPILabels{"method": "eth_estimateGas"}
	EthGasPriceLabel         = EthAPILabels{"method": "eth_gasPrice"}
	EthGetBalanceLabel       = EthAPILabels{"method": "eth_getBalance"}
	EthGetBlockByHashLabel   = EthAPILabels{"method": "eth_getBlockByHash"}
	EthGetBlockByNumberLabel = EthAPILabels{"method": "eth_getBlockByNumber"}

	EthGetBlockTransactionCountByNumberLabel = EthAPILabels{"method": "eth_getBlockTransactionCountByNumber"}

	EthGetCodeLabel          = EthAPILabels{"method": "eth_getCode"}
	EthGetFilterChangesLabel = EthAPILabels{"method": "eth_getFilterChanges"}
	EthGetFilterLogsLabel    = EthAPILabels{"method": "eth_getFilterLogs"}
	EthGetLogsLabel          = EthAPILabels{"method": "eth_getLogs"}
	EthGetStorageAtLabel     = EthAPILabels{"method": "eth_getStorageAt"}

	EthGetTransactionByHashLabel  = EthAPILabels{"method": "eth_getTransactionByHash"}
	EthGetTransactionCountLabel   = EthAPILabels{"method": "eth_getTransactionCount"}
	EthGetTransactionReceiptLabel = EthAPILabels{"method": "eth_getTransactionReceipt"}

	EthNewBlockFilterLabel = EthAPILabels{"method": "eth_newBlockFilter"}
	EthNewFilterLabel      = EthAPILabels{"method": "eth_newFilter"}

	EthSendRawTransactionLabel = EthAPILabels{"method": "eth_sendRawTransaction"}
	EthSyncingLabel            = EthAPILabels{"method": "eth_syncing"}

	EthUninstallFilterLabel = EthAPILabels{"method": "eth_uninstallFilter"}
	EthUnsubscribeLabel     = EthAPILabels{"method": "eth_unsubscribe"}
)

type NetAPILabels prometheus.Labels

var (
	NetVersionLabel   = NetAPILabels{"method": "net_version"}
	NetListeningLabel = NetAPILabels{"method": "net_listening"}
	NetPeerCountLabel = NetAPILabels{"method": "net_peerCount"}
)

type Web3APILabels prometheus.Labels

var (
	Web3ClientVersionLabel = Web3APILabels{"method": "web3_clientVersion"}
	Web3Sha3Label          = Web3APILabels{"method": "web3_sha3"}
)

type TxPoolAPILabels prometheus.Labels

var (
	TxPoolContentLabel = TxPoolAPILabels{"method": "txpool_content"}
	TxPoolInspectLabel = TxPoolAPILabels{"method": "txpool_inspect"}
	TxPoolStatusLabel  = TxPoolAPILabels{"method": "txpool_status"}
)

type DebugAPILabels prometheus.Labels

var (
	DebugTraceTransactionLabel = DebugAPILabels{"method": "debug_traceTransaction"}
)

// Metrics represents the jsonrpc metrics
type Metrics struct {
	// Requests number
	requests prometheus.Counter

	// Errors number
	errors prometheus.Counter

	// Requests duration (seconds)
	responseTime prometheus.Histogram

	// Eth metrics
	ethAPI *prometheus.CounterVec

	// Net metrics
	netAPI *prometheus.CounterVec

	// Web3 metrics
	web3API *prometheus.CounterVec

	// TxPool metrics
	txPoolAPI *prometheus.CounterVec

	// Debug metrics
	debugAPI *prometheus.CounterVec
}

func (m *Metrics) RequestsCounterInc() {
	metrics.CounterInc(m.requests)
}

func (m *Metrics) ErrorsCounterInc() {
	metrics.CounterInc(m.errors)
}

func (m *Metrics) ResponseTimeObserve(duration float64) {
	metrics.HistogramObserve(m.responseTime, duration)
}

func (m *Metrics) EthAPICounterInc(label EthAPILabels) {
	if m.ethAPI != nil {
		m.ethAPI.With((prometheus.Labels)(label)).Inc()
	}
}

func (m *Metrics) NetAPICounterInc(label NetAPILabels) {
	if m.netAPI != nil {
		m.netAPI.With((prometheus.Labels)(label)).Inc()
	}
}

func (m *Metrics) Web3APICounterInc(label Web3APILabels) {
	if m.web3API != nil {
		m.web3API.With((prometheus.Labels)(label)).Inc()
	}
}

func (m *Metrics) TxPoolAPICounterInc(label TxPoolAPILabels) {
	if m.txPoolAPI != nil {
		m.txPoolAPI.With((prometheus.Labels)(label)).Inc()
	}
}

func (m *Metrics) DebugAPICounterInc(label DebugAPILabels) {
	if m.debugAPI != nil {
		m.debugAPI.With((prometheus.Labels)(label)).Inc()
	}
}

// GetPrometheusMetrics return the blockchain metrics instance
func GetPrometheusMetrics(namespace string, labelsWithValues ...string) *Metrics {
	constLabels := metrics.ParseLables(labelsWithValues...)

	m := &Metrics{
		requests: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   "jsonrpc",
			Name:        "requests",
			Help:        "Requests number",
			ConstLabels: constLabels,
		}),
		errors: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   "jsonrpc",
			Name:        "request_errors",
			Help:        "Request errors number",
			ConstLabels: constLabels,
		}),
		responseTime: prometheus.NewHistogram(prometheus.HistogramOpts{
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
			ConstLabels: constLabels,
		}),
		ethAPI: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   "jsonrpc",
			Name:        "eth_api_requests",
			Help:        "eth api requests",
			ConstLabels: constLabels,
		}, []string{"method"}),
		netAPI: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   "jsonrpc",
			Name:        "net_api_requests",
			Help:        "net api requests",
			ConstLabels: constLabels,
		}, []string{"method"}),
		web3API: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   "jsonrpc",
			Name:        "web3_api_requests",
			Help:        "web3 api requests",
			ConstLabels: constLabels,
		}, []string{"method"}),
		txPoolAPI: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   "jsonrpc",
			Name:        "txpool_api_requests",
			Help:        "TxPool api requests",
			ConstLabels: constLabels,
		}, []string{"method"}),
		debugAPI: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   "jsonrpc",
			Name:        "debug_api_requests",
			Help:        "debug api requests",
			ConstLabels: constLabels,
		}, []string{"method"}),
	}

	prometheus.MustRegister(
		m.requests,
		m.errors,
		m.responseTime,
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
	return &Metrics{}
}

// NewDummyMetrics will return the no nil jsonrpc metrics
// TODO: use generic replace this in golang 1.18
func NewDummyMetrics(metrics *Metrics) *Metrics {
	if metrics != nil {
		return metrics
	}

	return NilMetrics()
}
