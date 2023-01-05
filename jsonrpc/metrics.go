package jsonrpc

import (
	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/discard"
	prometheus "github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

type EthAPILabels stdprometheus.Labels

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

type NetAPILabels stdprometheus.Labels

var (
	NetVersionLabel   = NetAPILabels{"method": "net_version"}
	NetListeningLabel = NetAPILabels{"method": "net_listening"}
	NetPeerCountLabel = NetAPILabels{"method": "net_peerCount"}
)

type Web3APILabels stdprometheus.Labels

var (
	Web3ClientVersionLabel = Web3APILabels{"method": "web3_clientVersion"}
	Web3Sha3Label          = Web3APILabels{"method": "web3_sha3"}
)

type TxPoolAPILabels stdprometheus.Labels

var (
	TxPoolContentLabel = TxPoolAPILabels{"method": "txpool_content"}
	TxPoolInspectLabel = TxPoolAPILabels{"method": "txpool_inspect"}
	TxPoolStatusLabel  = TxPoolAPILabels{"method": "txpool_status"}
)

type DebugAPILabels stdprometheus.Labels

var (
	DebugTraceTransactionLabel = DebugAPILabels{"method": "debug_traceTransaction"}
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

func (m *Metrics) EthAPICounterInc(label EthAPILabels) {
	if m.ethAPI != nil {
		m.ethAPI.With((stdprometheus.Labels)(label)).Inc()
	}
}

func (m *Metrics) NetAPICounterInc(label NetAPILabels) {
	if m.netAPI != nil {
		m.netAPI.With((stdprometheus.Labels)(label)).Inc()
	}
}

func (m *Metrics) Web3APICounterInc(label Web3APILabels) {
	if m.web3API != nil {
		m.web3API.With((stdprometheus.Labels)(label)).Inc()
	}
}

func (m *Metrics) TxPoolAPICounterInc(label TxPoolAPILabels) {
	if m.txPoolAPI != nil {
		m.txPoolAPI.With((stdprometheus.Labels)(label)).Inc()
	}
}

func (m *Metrics) DebugAPICounterInc(label DebugAPILabels) {
	if m.debugAPI != nil {
		m.debugAPI.With((stdprometheus.Labels)(label)).Inc()
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
