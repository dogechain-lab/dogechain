package abis

import (
	"github.com/umbracle/go-web3/abi"
)

// Predeployed system contract ABI
var StakingABI = abi.MustNewABI(StakingJSONABI)

// Temporarily deployed contract ABI
var StressTestABI = abi.MustNewABI(StressTestJSONABI)
