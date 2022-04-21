package abis

import (
	"github.com/umbracle/go-web3/abi"
)

// Predeployed system contract ABI
var (
	// staking contract (0x0000000000000000000000000000000000001001) abi
	StakingABI = abi.MustNewABI(StakingJSONABI)
	// bridge contract (0x0000000000000000000000000000000000001002) abi
	BridgeABI = abi.MustNewABI(BridgeJSONABI)
	// vault contract (0x0000000000000000000000000000000000001003) abi
	VaultABI = abi.MustNewABI(VaultJSONABI)
)

// Temporarily deployed contract ABI
var StressTestABI = abi.MustNewABI(StressTestJSONABI)
