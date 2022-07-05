package command

import "github.com/dogechain-lab/dogechain/server"

const (
	DefaultGenesisFileName       = "genesis.json"
	DefaultChainName             = "dogechain"
	DefaultChainID               = 100
	DefaultPremineBalance        = "0x3635C9ADC5DEA00000" // 1000 ETH
	DefaultConsensus             = server.IBFTConsensus
	DefaultPriceLimit            = 0
	DefaultMaxSlots              = 4096
	DefaultMaxAccountDemotions   = 10      // account demotion counter limit
	DefaultPruneTickSeconds      = 300     // ticker duration for pruning account future transactions
	DefaultPromoteOutdateSeconds = 1800    // not promoted account for a long time would be pruned
	DefaultGenesisGasUsed        = 458752  // 0x70000
	DefaultGenesisGasLimit       = 5242880 // 0x500000
)

const (
	JSONOutputFlag     = "json"
	GRPCAddressFlag    = "grpc-address"
	JSONRPCFlag        = "jsonrpc"
	GraphQLAddressFlag = "graphql-address"
)

// Legacy flag that needs to be present to preserve backwards
// compatibility with running clients
const (
	GRPCAddressFlagLEGACY = "grpc"
)
