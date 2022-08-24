package jsonrpc

const (
	// maximum length allowed for json_rpc batch requests
	DefaultJSONRPCBatchRequestLimit uint64 = 1
	// maximum block range allowed for json_rpc requests with fromBlock/toBlock values (e.g. eth_getLogs)
	DefaultJSONRPCBlockRangeLimit uint64 = 100
)
