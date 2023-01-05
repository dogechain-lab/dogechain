package server

import (
	"github.com/prometheus/client_golang/prometheus"
)

type JSONRPCStoreMetrics struct {
	counter *prometheus.CounterVec
}

// GetNonce api calls
func (m *JSONRPCStoreMetrics) GetNonceInc() {
	if m.counter != nil {
		m.counter.With(prometheus.Labels{"method": "GetNonce"}).Inc()
	}
}

// AddTx api calls
func (m *JSONRPCStoreMetrics) AddTxInc() {
	if m.counter != nil {
		m.counter.With(prometheus.Labels{"method": "AddTx"}).Inc()
	}
}

// GetPendingTx api calls
func (m *JSONRPCStoreMetrics) GetPendingTxInc() {
	if m.counter != nil {
		m.counter.With(prometheus.Labels{"method": "GetPendingTx"}).Inc()
	}
}

// GetAccount api calls
func (m *JSONRPCStoreMetrics) GetAccountInc() {
	if m.counter != nil {
		m.counter.With(prometheus.Labels{"method": "GetAccount"}).Inc()
	}
}

// GetGetStorage api calls
func (m *JSONRPCStoreMetrics) GetStorageInc() {
	if m.counter != nil {
		m.counter.With(prometheus.Labels{"method": "GetStorage"}).Inc()
	}
}

// GetForksInTime api calls
func (m *JSONRPCStoreMetrics) GetForksInTimeInc() {
	if m.counter != nil {
		m.counter.With(prometheus.Labels{"method": "GetForksInTime"}).Inc()
	}
}

// GetCode api calls
func (m *JSONRPCStoreMetrics) GetCodeInc() {
	if m.counter != nil {
		m.counter.With(prometheus.Labels{"method": "GetCode"}).Inc()
	}
}

// Header api calls
func (m *JSONRPCStoreMetrics) HeaderInc() {
	if m.counter != nil {
		m.counter.With(prometheus.Labels{"method": "Header"}).Inc()
	}
}

// GetHeaderByNumber api calls
func (m *JSONRPCStoreMetrics) GetHeaderByNumberInc() {
	if m.counter != nil {
		m.counter.With(prometheus.Labels{"method": "GetHeaderByNumber"}).Inc()
	}
}

// GetHeaderByHash api calls
func (m *JSONRPCStoreMetrics) GetHeaderByHashInc() {
	if m.counter != nil {
		m.counter.With(prometheus.Labels{"method": "GetHeaderByHash"}).Inc()
	}
}

// GetBlockByHash api calls
func (m *JSONRPCStoreMetrics) GetBlockByHashInc() {
	if m.counter != nil {
		m.counter.With(prometheus.Labels{"method": "GetBlockByHash"}).Inc()
	}
}

// GetBlockByNumber api calls
func (m *JSONRPCStoreMetrics) GetBlockByNumberInc() {
	if m.counter != nil {
		m.counter.With(prometheus.Labels{"method": "GetBlockByNumber"}).Inc()
	}
}

// ReadTxLookup api calls
func (m *JSONRPCStoreMetrics) ReadTxLookupInc() {
	if m.counter != nil {
		m.counter.With(prometheus.Labels{"method": "ReadTxLookup"}).Inc()
	}
}

// GetReceiptsByHash api calls
func (m *JSONRPCStoreMetrics) GetReceiptsByHashInc() {
	if m.counter != nil {
		m.counter.With(prometheus.Labels{"method": "GetReceiptsByHash"}).Inc()
	}
}

// GetAvgGasPrice api calls
func (m *JSONRPCStoreMetrics) GetAvgGasPriceInc() {
	if m.counter != nil {
		m.counter.With(prometheus.Labels{"method": "GetAvgGasPrice"}).Inc()
	}
}

// ApplyTxn api calls
func (m *JSONRPCStoreMetrics) ApplyTxnInc() {
	if m.counter != nil {
		m.counter.With(prometheus.Labels{"method": "ApplyTxn"}).Inc()
	}
}

// GetSyncProgression api calls
func (m *JSONRPCStoreMetrics) GetSyncProgressionInc() {
	if m.counter != nil {
		m.counter.With(prometheus.Labels{"method": "GetSyncProgression"}).Inc()
	}
}

// StateAtTransaction api calls
func (m *JSONRPCStoreMetrics) StateAtTransactionInc() {
	if m.counter != nil {
		m.counter.With(prometheus.Labels{"method": "StateAtTransaction"}).Inc()
	}
}

// PeerCount api calls
func (m *JSONRPCStoreMetrics) PeerCountInc() {
	if m.counter != nil {
		m.counter.With(prometheus.Labels{"method": "PeerCount"}).Inc()
	}
}

// GetTxs api calls
func (m *JSONRPCStoreMetrics) GetTxsInc() {
	if m.counter != nil {
		m.counter.With(prometheus.Labels{"method": "GetTxs"}).Inc()
	}
}

// GetCapacity api calls
func (m *JSONRPCStoreMetrics) GetCapacityInc() {
	if m.counter != nil {
		m.counter.With(prometheus.Labels{"method": "GetCapacity"}).Inc()
	}
}

// SubscribeEvents api calls
func (m *JSONRPCStoreMetrics) SubscribeEventsInc() {
	if m.counter != nil {
		m.counter.With(prometheus.Labels{"method": "SubscribeEvents"}).Inc()
	}
}

// NewJSONRPCStoreMetrics return the JSONRPCStore metrics instance
func NewJSONRPCStoreMetrics(namespace string, labelsWithValues ...string) *JSONRPCStoreMetrics {
	constLabels := map[string]string{}
	for i := 1; i < len(labelsWithValues); i += 2 {
		constLabels[labelsWithValues[i-1]] = labelsWithValues[i]
	}

	m := &JSONRPCStoreMetrics{
		counter: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   "jsonrpc_store",
			Name:        "api_call_counter",
			Help:        "api call counter",
			ConstLabels: constLabels,
		}, []string{"method"}),
	}

	prometheus.MustRegister(m.counter)

	return m
}

// JSONRPCStoreNilMetrics will return the non operational jsonrpc metrics
func JSONRPCStoreNilMetrics() *JSONRPCStoreMetrics {
	return &JSONRPCStoreMetrics{
		counter: nil,
	}
}
