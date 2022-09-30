package server

import (
	"github.com/dogechain-lab/dogechain/blockchain"
	"github.com/dogechain-lab/dogechain/consensus"
	"github.com/dogechain-lab/dogechain/jsonrpc"
	"github.com/dogechain-lab/dogechain/network"
	"github.com/dogechain-lab/dogechain/txpool"
)

// serverMetrics holds the metric instances of all sub systems
type serverMetrics struct {
	blockchain *blockchain.Metrics
	consensus  *consensus.Metrics
	network    *network.Metrics
	txpool     *txpool.Metrics
	jsonrpc    *jsonrpc.Metrics
}

// metricProvider serverMetric instance for the given ChainID and nameSpace
func metricProvider(nameSpace string, chainID string, metricsRequired bool) *serverMetrics {
	if metricsRequired {
		return &serverMetrics{
			blockchain: blockchain.GetPrometheusMetrics(nameSpace, "chain_id", chainID),
			consensus:  consensus.GetPrometheusMetrics(nameSpace, "chain_id", chainID),
			network:    network.GetPrometheusMetrics(nameSpace, "chain_id", chainID),
			txpool:     txpool.GetPrometheusMetrics(nameSpace, "chain_id", chainID),
			jsonrpc:    jsonrpc.GetPrometheusMetrics(nameSpace, "chain_id", chainID),
		}
	}

	return &serverMetrics{
		blockchain: blockchain.NilMetrics(),
		consensus:  consensus.NilMetrics(),
		network:    network.NilMetrics(),
		txpool:     txpool.NilMetrics(),
		jsonrpc:    jsonrpc.NilMetrics(),
	}
}
