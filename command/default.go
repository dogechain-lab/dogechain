package command

import "github.com/dogechain-lab/jury/server"

const (
	DefaultGenesisFileName     = "genesis.json"
	DefaultChainName           = "dogechain"
	DefaultChainID             = 100
	DefaultPremineBalance      = "0x3635C9ADC5DEA00000" // 1000 ETH
	DefaultConsensus           = server.IBFTConsensus
	DefaultPriceLimit          = 0
	DefaultMaxSlots            = 4096
	DefaultMaxAccountDemotions = 10      // account demotion counter limit
	DefaultGenesisGasUsed      = 458752  // 0x70000
	DefaultGenesisGasLimit     = 5242880 // 0x500000
)

const (
	JSONOutputFlag  = "json"
	GRPCAddressFlag = "grpc-address"
	JSONRPCFlag     = "jsonrpc"
)

// Legacy flag that needs to be present to preserve backwards
// compatibility with running clients
const (
	GRPCAddressFlagLEGACY = "grpc"
)
