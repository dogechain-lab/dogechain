package server

import (
	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/discard"

	prometheus "github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

type JSONRPCStoreMetrics struct {

	// GetNonce api calls
	GetNonce metrics.Counter

	// AddTx api calls
	AddTx metrics.Counter

	// GetPendingTx api calls
	GetPendingTx metrics.Counter

	// GetAccount api calls
	GetAccount metrics.Counter

	// GetGetStorage api calls
	GetStorage metrics.Counter

	// GetForksInTime api calls
	GetForksInTime metrics.Counter

	// GetCode api calls
	GetCode metrics.Counter

	// Header api calls
	Header metrics.Counter

	// GetHeaderByNumber api calls
	GetHeaderByNumber metrics.Counter

	// GetHeaderByHash api calls
	GetHeaderByHash metrics.Counter

	// GetBlockByHash api calls
	GetBlockByHash metrics.Counter

	// GetBlockByNumber api calls
	GetBlockByNumber metrics.Counter

	// ReadTxLookup api calls
	ReadTxLookup metrics.Counter

	// GetReceiptsByHash api calls
	GetReceiptsByHash metrics.Counter

	// GetAvgGasPrice api calls
	GetAvgGasPrice metrics.Counter

	// ApplyTxn api calls
	ApplyTxn metrics.Counter

	// GetSyncProgression api calls
	GetSyncProgression metrics.Counter

	// StateAtTransaction api calls
	StateAtTransaction metrics.Counter

	// PeerCount api calls
	PeerCount metrics.Counter

	// GetTxs api calls
	GetTxs metrics.Counter

	// GetCapacity api calls
	GetCapacity metrics.Counter

	// SubscribeEvents api calls
	SubscribeEvents metrics.Counter
}

// NewJSONRPCStoreMetrics return the JSONRPCStore metrics instance
func NewJSONRPCStoreMetrics(namespace string, labelsWithValues ...string) *JSONRPCStoreMetrics {
	labels := []string{}

	for i := 0; i < len(labelsWithValues); i += 2 {
		labels = append(labels, labelsWithValues[i])
	}

	return &JSONRPCStoreMetrics{
		// GetNonce api calls
		GetNonce: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "jsonrpc_store",
			Name:      "get_nonce",
			Help:      "GetNonce api calls",
		}, labels).With(labelsWithValues...),

		// AddTx api calls
		AddTx: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "jsonrpc_store",
			Name:      "add_tx",
			Help:      "AddTx api calls",
		}, labels).With(labelsWithValues...),

		// GetPendingTx api calls
		GetPendingTx: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "jsonrpc_store",
			Name:      "get_pending_tx",
			Help:      "GetPendingTx api calls",
		}, labels).With(labelsWithValues...),

		// GetAccount api calls
		GetAccount: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "jsonrpc_store",
			Name:      "get_account",
			Help:      "GetAccount api calls",
		}, labels).With(labelsWithValues...),

		// GetGetStorage api calls
		GetStorage: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "jsonrpc_store",
			Name:      "get_storage",
			Help:      "GetStorage api calls",
		}, labels).With(labelsWithValues...),

		// GetForksInTime api calls
		GetForksInTime: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "jsonrpc_store",
			Name:      "get_forks_in_time",
			Help:      "GetForksInTime api calls",
		}, labels).With(labelsWithValues...),

		// GetCode api calls
		GetCode: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "jsonrpc_store",
			Name:      "get_code",
			Help:      "GetCode api calls",
		}, labels).With(labelsWithValues...),

		// Header api calls
		Header: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "jsonrpc_store",
			Name:      "header",
			Help:      "Header api calls",
		}, labels).With(labelsWithValues...),

		// GetHeaderByNumber api calls
		GetHeaderByNumber: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "jsonrpc_store",
			Name:      "get_header_by_number",
			Help:      "GetHeaderByNumber api calls",
		}, labels).With(labelsWithValues...),

		// GetHeaderByHash api calls
		GetHeaderByHash: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "jsonrpc_store",
			Name:      "get_header_by_hash",
			Help:      "GetHeaderByHash api calls",
		}, labels).With(labelsWithValues...),

		// GetBlockByHash api calls
		GetBlockByHash: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "jsonrpc_store",
			Name:      "get_block_by_hash",
			Help:      "GetBlockByHash api calls",
		}, labels).With(labelsWithValues...),

		// GetBlockByNumber api calls
		GetBlockByNumber: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "jsonrpc_store",
			Name:      "get_block_by_number",
			Help:      "GetBlockByNumber api calls",
		}, labels).With(labelsWithValues...),

		// ReadTxLookup api calls
		ReadTxLookup: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "jsonrpc_store",
			Name:      "read_tx_lookup",
			Help:      "ReadTxLookup api calls",
		}, labels).With(labelsWithValues...),

		// GetReceiptsByHash api calls
		GetReceiptsByHash: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "jsonrpc_store",
			Name:      "get_receipts_by_hash",
			Help:      "GetReceiptsByHash api calls",
		}, labels).With(labelsWithValues...),

		// GetAvgGasPrice api calls
		GetAvgGasPrice: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "jsonrpc_store",
			Name:      "get_avg_gas_price",
			Help:      "GetAvgGasPrice api calls",
		}, labels).With(labelsWithValues...),

		// ApplyTxn api calls
		ApplyTxn: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "jsonrpc_store",
			Name:      "apply_txn",
			Help:      "ApplyTxn api calls",
		}, labels).With(labelsWithValues...),

		// GetSyncProgression api calls
		GetSyncProgression: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "jsonrpc_store",
			Name:      "get_sync_progression",
			Help:      "GetSyncProgression api calls",
		}, labels).With(labelsWithValues...),

		// StateAtTransaction api calls
		StateAtTransaction: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "jsonrpc_store",
			Name:      "state_at_transaction",
			Help:      "StateAtTransaction api calls",
		}, labels).With(labelsWithValues...),

		// PeerCount api calls
		PeerCount: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "jsonrpc_store",
			Name:      "peer_count",
			Help:      "PeerCount api calls",
		}, labels).With(labelsWithValues...),

		// GetTxs api calls
		GetTxs: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "jsonrpc_store",
			Name:      "get_txs",
			Help:      "GetTxs api calls",
		}, labels).With(labelsWithValues...),

		// GetCapacity api calls
		GetCapacity: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "jsonrpc_store",
			Name:      "get_capacity",
			Help:      "GetCapacity api calls",
		}, labels).With(labelsWithValues...),

		// SubscribeEvents api calls
		SubscribeEvents: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "jsonrpc_store",
			Name:      "subscribe_events",
			Help:      "SubscribeEvents api calls",
		}, labels).With(labelsWithValues...),
	}
}

// JSONRPCStoreNilMetrics will return the non operational jsonrpc metrics
func JSONRPCStoreNilMetrics() *JSONRPCStoreMetrics {
	return &JSONRPCStoreMetrics{

		// GetNonce api calls
		GetNonce: discard.NewCounter(),

		// AddTx api calls
		AddTx: discard.NewCounter(),

		// GetPendingTx api calls
		GetPendingTx: discard.NewCounter(),

		// GetAccount api calls
		GetAccount: discard.NewCounter(),

		// GetGetStorage api calls
		GetStorage: discard.NewCounter(),

		// GetForksInTime api calls
		GetForksInTime: discard.NewCounter(),

		// GetCode api calls
		GetCode: discard.NewCounter(),

		// Header api calls
		Header: discard.NewCounter(),

		// GetHeaderByNumber api calls
		GetHeaderByNumber: discard.NewCounter(),

		// GetHeaderByHash api calls
		GetHeaderByHash: discard.NewCounter(),

		// GetBlockByHash api calls
		GetBlockByHash: discard.NewCounter(),

		// GetBlockByNumber api calls
		GetBlockByNumber: discard.NewCounter(),

		// ReadTxLookup api calls
		ReadTxLookup: discard.NewCounter(),

		// GetReceiptsByHash api calls
		GetReceiptsByHash: discard.NewCounter(),

		// GetAvgGasPrice api calls
		GetAvgGasPrice: discard.NewCounter(),

		// ApplyTxn api calls
		ApplyTxn: discard.NewCounter(),

		// GetSyncProgression api calls
		GetSyncProgression: discard.NewCounter(),

		// StateAtTransaction api calls
		StateAtTransaction: discard.NewCounter(),

		// PeerCount api calls
		PeerCount: discard.NewCounter(),

		// GetTxs api calls
		GetTxs: discard.NewCounter(),

		// GetCapacity api calls
		GetCapacity: discard.NewCounter(),

		// SubscribeEvents api calls
		SubscribeEvents: discard.NewCounter(),
	}
}
