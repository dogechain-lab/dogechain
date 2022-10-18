package abis

const ValidatorSetJSONABI = `[
    {
        "inputs":
        [],
        "stateMutability": "nonpayable",
        "type": "constructor"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "uint256",
                "name": "prevValue",
                "type": "uint256"
            },
            {
                "indexed": true,
                "internalType": "uint256",
                "name": "newValue",
                "type": "uint256"
            }
        ],
        "name": "ActiveValidatorsLengthChanged",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "address",
                "name": "sender",
                "type": "address"
            },
            {
                "indexed": true,
                "internalType": "address",
                "name": "validator",
                "type": "address"
            },
            {
                "indexed": false,
                "internalType": "uint256",
                "name": "reward",
                "type": "uint256"
            }
        ],
        "name": "Claimed",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "address",
                "name": "sender",
                "type": "address"
            },
            {
                "indexed": true,
                "internalType": "address",
                "name": "validator",
                "type": "address"
            },
            {
                "indexed": false,
                "internalType": "uint256",
                "name": "amount",
                "type": "uint256"
            }
        ],
        "name": "Delegated",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "uint256",
                "name": "prevValue",
                "type": "uint256"
            },
            {
                "indexed": true,
                "internalType": "uint256",
                "name": "newValue",
                "type": "uint256"
            }
        ],
        "name": "EpochBlockIntervalChanged",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "uint256",
                "name": "prevValue",
                "type": "uint256"
            },
            {
                "indexed": true,
                "internalType": "uint256",
                "name": "newValue",
                "type": "uint256"
            }
        ],
        "name": "FelonyThresholdChanged",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "uint256",
                "name": "prevValue",
                "type": "uint256"
            },
            {
                "indexed": true,
                "internalType": "uint256",
                "name": "newValue",
                "type": "uint256"
            }
        ],
        "name": "MinDelegatorStakeAmountChanged",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "uint256",
                "name": "prevValue",
                "type": "uint256"
            },
            {
                "indexed": true,
                "internalType": "uint256",
                "name": "newValue",
                "type": "uint256"
            }
        ],
        "name": "MinStakePeriodChanged",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "uint256",
                "name": "prevValue",
                "type": "uint256"
            },
            {
                "indexed": true,
                "internalType": "uint256",
                "name": "newValue",
                "type": "uint256"
            }
        ],
        "name": "MinValidatorStakeAmountChanged",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "uint256",
                "name": "prevValue",
                "type": "uint256"
            },
            {
                "indexed": true,
                "internalType": "uint256",
                "name": "newValue",
                "type": "uint256"
            }
        ],
        "name": "MisdemeanorThresholdChanged",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "address",
                "name": "prevValue",
                "type": "address"
            },
            {
                "indexed": true,
                "internalType": "address",
                "name": "newValue",
                "type": "address"
            }
        ],
        "name": "OwnershipTransferred",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "uint256",
                "name": "prevValue",
                "type": "uint256"
            },
            {
                "indexed": true,
                "internalType": "uint256",
                "name": "newValue",
                "type": "uint256"
            }
        ],
        "name": "RewardPerBlockChanged",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "address",
                "name": "prevValue",
                "type": "address"
            },
            {
                "indexed": true,
                "internalType": "address",
                "name": "newValue",
                "type": "address"
            }
        ],
        "name": "RewardTokenChanged",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "address",
                "name": "prevValue",
                "type": "address"
            },
            {
                "indexed": true,
                "internalType": "address",
                "name": "newValue",
                "type": "address"
            }
        ],
        "name": "StakeTokenChanged",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "address",
                "name": "sender",
                "type": "address"
            },
            {
                "indexed": true,
                "internalType": "address",
                "name": "validator",
                "type": "address"
            },
            {
                "indexed": false,
                "internalType": "uint256",
                "name": "amount",
                "type": "uint256"
            },
            {
                "indexed": false,
                "internalType": "uint256",
                "name": "reward",
                "type": "uint256"
            }
        ],
        "name": "Undelegated",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "address",
                "name": "validator",
                "type": "address"
            }
        ],
        "name": "ValidatorAdded",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "address",
                "name": "validator",
                "type": "address"
            },
            {
                "indexed": false,
                "internalType": "uint256",
                "name": "amount",
                "type": "uint256"
            },
            {
                "indexed": false,
                "internalType": "uint256",
                "name": "number",
                "type": "uint256"
            }
        ],
        "name": "ValidatorDeposited",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "uint256",
                "name": "prevValue",
                "type": "uint256"
            },
            {
                "indexed": true,
                "internalType": "uint256",
                "name": "newValue",
                "type": "uint256"
            }
        ],
        "name": "ValidatorJailEpochLengthChanged",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "address",
                "name": "validator",
                "type": "address"
            },
            {
                "indexed": false,
                "internalType": "uint256",
                "name": "epoch",
                "type": "uint256"
            }
        ],
        "name": "ValidatorJailed",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "address",
                "name": "validator",
                "type": "address"
            },
            {
                "indexed": false,
                "internalType": "uint256",
                "name": "status",
                "type": "uint256"
            }
        ],
        "name": "ValidatorModified",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "address",
                "name": "validator",
                "type": "address"
            },
            {
                "indexed": false,
                "internalType": "uint256",
                "name": "epoch",
                "type": "uint256"
            }
        ],
        "name": "ValidatorReleased",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "address",
                "name": "validator",
                "type": "address"
            }
        ],
        "name": "ValidatorRemoved",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "address",
                "name": "validator",
                "type": "address"
            },
            {
                "indexed": false,
                "internalType": "uint256",
                "name": "slashes",
                "type": "uint256"
            },
            {
                "indexed": false,
                "internalType": "uint256",
                "name": "epoch",
                "type": "uint256"
            }
        ],
        "name": "ValidatorSlashed",
        "type": "event"
    },
    {
        "inputs":
        [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "name": "_validators",
        "outputs":
        [
            {
                "internalType": "address",
                "name": "",
                "type": "address"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "validatorAddress",
                "type": "address"
            }
        ],
        "name": "activateValidator",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "validatorAddress",
                "type": "address"
            }
        ],
        "name": "claim",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "currentEpoch",
        "outputs":
        [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "validatorAddress",
                "type": "address"
            },
            {
                "internalType": "uint256",
                "name": "amount",
                "type": "uint256"
            }
        ],
        "name": "delegate",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "",
                "type": "address"
            }
        ],
        "name": "delegatorStakeAmount",
        "outputs":
        [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "",
                "type": "address"
            },
            {
                "internalType": "address",
                "name": "",
                "type": "address"
            }
        ],
        "name": "delegators",
        "outputs":
        [
            {
                "internalType": "uint256",
                "name": "amount",
                "type": "uint256"
            },
            {
                "internalType": "uint256",
                "name": "rewardDebt",
                "type": "uint256"
            },
            {
                "internalType": "uint256",
                "name": "changedAt",
                "type": "uint256"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "deposit",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "detroitMigration",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "validatorAddress",
                "type": "address"
            }
        ],
        "name": "disableValidator",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "name": "exists",
        "outputs":
        [
            {
                "internalType": "bool",
                "name": "",
                "type": "bool"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "validatorAddress",
                "type": "address"
            }
        ],
        "name": "forceUnJailValidator",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "getActiveValidatorsLength",
        "outputs":
        [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "validator",
                "type": "address"
            },
            {
                "internalType": "address",
                "name": "account",
                "type": "address"
            }
        ],
        "name": "getDelegatorStakedAmount",
        "outputs":
        [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "validatorAddress",
                "type": "address"
            },
            {
                "internalType": "address",
                "name": "account",
                "type": "address"
            }
        ],
        "name": "getDelegatorStakedReward",
        "outputs":
        [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "account",
                "type": "address"
            }
        ],
        "name": "getDelegatorTotalStakedAmount",
        "outputs":
        [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "getDelegatorsLength",
        "outputs":
        [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "getEpochBlockInterval",
        "outputs":
        [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "getFelonyThreshold",
        "outputs":
        [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "getMinDelegatorStakeAmount",
        "outputs":
        [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "getMinStakePeriod",
        "outputs":
        [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "getMinValidatorStakeAmount",
        "outputs":
        [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "getMisdemeanorThreshold",
        "outputs":
        [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "getRewardPerBlock",
        "outputs":
        [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "getStakeToken",
        "outputs":
        [
            {
                "internalType": "address",
                "name": "",
                "type": "address"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "validator",
                "type": "address"
            }
        ],
        "name": "getValidatorDelegatorLength",
        "outputs":
        [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "validator",
                "type": "address"
            },
            {
                "internalType": "uint256",
                "name": "offset",
                "type": "uint256"
            },
            {
                "internalType": "uint256",
                "name": "limit",
                "type": "uint256"
            }
        ],
        "name": "getValidatorDelegators",
        "outputs":
        [
            {
                "internalType": "address[]",
                "name": "",
                "type": "address[]"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "getValidatorJailEpochLength",
        "outputs":
        [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "getValidatorLength",
        "outputs":
        [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "validator",
                "type": "address"
            }
        ],
        "name": "getValidatorStakedAmount",
        "outputs":
        [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "validator",
                "type": "address"
            }
        ],
        "name": "getValidatorStakedReward",
        "outputs":
        [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "validator",
                "type": "address"
            }
        ],
        "name": "isValidator",
        "outputs":
        [
            {
                "internalType": "bool",
                "name": "",
                "type": "bool"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "validatorAddress",
                "type": "address"
            }
        ],
        "name": "jailValidator",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "nextEpoch",
        "outputs":
        [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "owner",
        "outputs":
        [
            {
                "internalType": "address",
                "name": "",
                "type": "address"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "paused",
        "outputs":
        [
            {
                "internalType": "bool",
                "name": "",
                "type": "bool"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "string",
                "name": "name",
                "type": "string"
            },
            {
                "internalType": "address",
                "name": "validatorAddress",
                "type": "address"
            },
            {
                "internalType": "uint256",
                "name": "amount",
                "type": "uint256"
            }
        ],
        "name": "registerValidator",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "validatorAddress",
                "type": "address"
            }
        ],
        "name": "releaseValidatorFromJail",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "releasedReward",
        "outputs":
        [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "validatorAddress",
                "type": "address"
            }
        ],
        "name": "removeValidator",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "rewardToken",
        "outputs":
        [
            {
                "internalType": "address",
                "name": "",
                "type": "address"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "uint256",
                "name": "newValue",
                "type": "uint256"
            }
        ],
        "name": "setActiveValidatorsLength",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "uint256",
                "name": "newValue",
                "type": "uint256"
            }
        ],
        "name": "setEpochBlockInterval",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "uint256",
                "name": "newValue",
                "type": "uint256"
            }
        ],
        "name": "setFelonyThreshold",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "uint256",
                "name": "newValue",
                "type": "uint256"
            }
        ],
        "name": "setMinDelegatorStakeAmount",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "uint32",
                "name": "newValue",
                "type": "uint32"
            }
        ],
        "name": "setMinStakePeriod",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "uint256",
                "name": "newValue",
                "type": "uint256"
            }
        ],
        "name": "setMinValidatorStakeAmount",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "uint256",
                "name": "newValue",
                "type": "uint256"
            }
        ],
        "name": "setMisdemeanorThreshold",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "setPause",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "uint256",
                "name": "newValue",
                "type": "uint256"
            }
        ],
        "name": "setRewardPerBlock",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "newValue",
                "type": "address"
            }
        ],
        "name": "setRewardToken",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "newValue",
                "type": "address"
            }
        ],
        "name": "setStakeToken",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "uint256",
                "name": "newValue",
                "type": "uint256"
            }
        ],
        "name": "setValidatorJailEpochLength",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "validatorAddress",
                "type": "address"
            }
        ],
        "name": "slash",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "stakedAmount",
        "outputs":
        [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "newOwner",
                "type": "address"
            }
        ],
        "name": "transferOwnership",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "validatorAddress",
                "type": "address"
            },
            {
                "internalType": "uint256",
                "name": "amount",
                "type": "uint256"
            }
        ],
        "name": "undelegate",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "",
                "type": "address"
            }
        ],
        "name": "validatorPools",
        "outputs":
        [
            {
                "internalType": "string",
                "name": "name",
                "type": "string"
            },
            {
                "internalType": "address",
                "name": "owner",
                "type": "address"
            },
            {
                "internalType": "uint256",
                "name": "jailedBefore",
                "type": "uint256"
            },
            {
                "internalType": "uint256",
                "name": "stakedAmount",
                "type": "uint256"
            },
            {
                "internalType": "uint256",
                "name": "stakedReward",
                "type": "uint256"
            },
            {
                "internalType": "uint256",
                "name": "stakedPerShare",
                "type": "uint256"
            },
            {
                "internalType": "uint256",
                "name": "slashesCount",
                "type": "uint256"
            },
            {
                "internalType": "enum ValidatorSet.Status",
                "name": "status",
                "type": "uint8"
            },
            {
                "internalType": "bool",
                "name": "jailed",
                "type": "bool"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "",
                "type": "address"
            }
        ],
        "name": "validatorSlashes",
        "outputs":
        [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "validators",
        "outputs":
        [
            {
                "internalType": "address[]",
                "name": "",
                "type": "address[]"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    }
]`

const BridgeJSONABI = `[
    {
        "inputs":
        [],
        "stateMutability": "nonpayable",
        "type": "constructor"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "address",
                "name": "sender",
                "type": "address"
            },
            {
                "indexed": true,
                "internalType": "uint256",
                "name": "amount",
                "type": "uint256"
            }
        ],
        "name": "Burned",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "address",
                "name": "receiver",
                "type": "address"
            },
            {
                "indexed": true,
                "internalType": "uint256",
                "name": "amount",
                "type": "uint256"
            },
            {
                "indexed": false,
                "internalType": "string",
                "name": "txid",
                "type": "string"
            },
            {
                "indexed": false,
                "internalType": "string",
                "name": "sender",
                "type": "string"
            }
        ],
        "name": "Deposited",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "address",
                "name": "sender",
                "type": "address"
            },
            {
                "indexed": true,
                "internalType": "uint256",
                "name": "oldMinimumThreshold",
                "type": "uint256"
            },
            {
                "indexed": true,
                "internalType": "uint256",
                "name": "newMinimumThreshold",
                "type": "uint256"
            }
        ],
        "name": "MinimumThresholdSet",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "address",
                "name": "previousOwner",
                "type": "address"
            },
            {
                "indexed": true,
                "internalType": "address",
                "name": "newOwner",
                "type": "address"
            }
        ],
        "name": "OwnershipTransferred",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "address",
                "name": "sender",
                "type": "address"
            },
            {
                "indexed": true,
                "internalType": "uint256",
                "name": "oldRate",
                "type": "uint256"
            },
            {
                "indexed": true,
                "internalType": "uint256",
                "name": "newRate",
                "type": "uint256"
            }
        ],
        "name": "RateSet",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "address",
                "name": "sender",
                "type": "address"
            },
            {
                "indexed": true,
                "internalType": "address",
                "name": "signer",
                "type": "address"
            }
        ],
        "name": "SignerDeleted",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "address",
                "name": "sender",
                "type": "address"
            },
            {
                "indexed": true,
                "internalType": "address",
                "name": "signer",
                "type": "address"
            }
        ],
        "name": "ValidatorAdded",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "address",
                "name": "sender",
                "type": "address"
            },
            {
                "indexed": true,
                "internalType": "uint256",
                "name": "amount",
                "type": "uint256"
            },
            {
                "indexed": true,
                "internalType": "uint256",
                "name": "fee",
                "type": "uint256"
            },
            {
                "indexed": false,
                "internalType": "string",
                "name": "receiver",
                "type": "string"
            }
        ],
        "name": "Withdrawn",
        "type": "event"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "account",
                "type": "address"
            }
        ],
        "name": "addSigner",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "allowance",
        "outputs":
        [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "account",
                "type": "address"
            },
            {
                "internalType": "uint256",
                "name": "amount",
                "type": "uint256"
            }
        ],
        "name": "burn",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "uint256",
                "name": "amount",
                "type": "uint256"
            }
        ],
        "name": "decreaseAllowance",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "account",
                "type": "address"
            }
        ],
        "name": "deleteSigner",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "receiver",
                "type": "address"
            },
            {
                "internalType": "uint256",
                "name": "amount",
                "type": "uint256"
            },
            {
                "internalType": "string",
                "name": "txid",
                "type": "string"
            },
            {
                "internalType": "string",
                "name": "sender",
                "type": "string"
            }
        ],
        "name": "deposit",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "getMinimumThreshold",
        "outputs":
        [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "getSigners",
        "outputs":
        [
            {
                "internalType": "address[]",
                "name": "",
                "type": "address[]"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "uint256",
                "name": "amount",
                "type": "uint256"
            }
        ],
        "name": "increaseAllowance",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "account",
                "type": "address"
            }
        ],
        "name": "isSigner",
        "outputs":
        [
            {
                "internalType": "bool",
                "name": "",
                "type": "bool"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "owner",
        "outputs":
        [
            {
                "internalType": "address",
                "name": "",
                "type": "address"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "paused",
        "outputs":
        [
            {
                "internalType": "bool",
                "name": "",
                "type": "bool"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "rate",
        "outputs":
        [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "renounceOwnership",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "uint256",
                "name": "newMinimumThreshold",
                "type": "uint256"
            }
        ],
        "name": "setMinimumThreshold",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "setPause",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "uint256",
                "name": "newRate",
                "type": "uint256"
            }
        ],
        "name": "setRate",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "receiver",
                "type": "address"
            },
            {
                "internalType": "uint256",
                "name": "amount",
                "type": "uint256"
            },
            {
                "internalType": "string",
                "name": "txid",
                "type": "string"
            },
            {
                "internalType": "string",
                "name": "sender",
                "type": "string"
            }
        ],
        "name": "signatures",
        "outputs":
        [
            {
                "internalType": "address[]",
                "name": "",
                "type": "address[]"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "totalSupply",
        "outputs":
        [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "newOwner",
                "type": "address"
            }
        ],
        "name": "transferOwnership",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "string",
                "name": "receiver",
                "type": "string"
            }
        ],
        "name": "withdraw",
        "outputs":
        [],
        "stateMutability": "payable",
        "type": "function"
    }
]`

const VaultJSONABI = `[
    {
        "inputs":
        [],
        "stateMutability": "nonpayable",
        "type": "constructor"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "address",
                "name": "previousOwner",
                "type": "address"
            },
            {
                "indexed": true,
                "internalType": "address",
                "name": "newOwner",
                "type": "address"
            }
        ],
        "name": "OwnershipTransferred",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "address",
                "name": "from",
                "type": "address"
            },
            {
                "indexed": true,
                "internalType": "uint256",
                "name": "amount",
                "type": "uint256"
            }
        ],
        "name": "ReceiveReward",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "address",
                "name": "to",
                "type": "address"
            },
            {
                "indexed": true,
                "internalType": "uint256",
                "name": "amount",
                "type": "uint256"
            }
        ],
        "name": "RewardTo",
        "type": "event"
    },
    {
        "inputs":
        [],
        "name": "balance",
        "outputs":
        [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address payable",
                "name": "to",
                "type": "address"
            },
            {
                "internalType": "uint256",
                "name": "amount",
                "type": "uint256"
            }
        ],
        "name": "claimRewards",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "owner",
        "outputs":
        [
            {
                "internalType": "address",
                "name": "",
                "type": "address"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs":
        [],
        "name": "renounceOwnership",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs":
        [
            {
                "internalType": "address",
                "name": "newOwner",
                "type": "address"
            }
        ],
        "name": "transferOwnership",
        "outputs":
        [],
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "stateMutability": "payable",
        "type": "receive"
    }
]`

const StressTestJSONABI = `[
    {
      "inputs": [],
      "stateMutability": "nonpayable",
      "type": "constructor"
    },
    {
      "anonymous": false,
      "inputs": [
        {
          "indexed": false,
          "internalType": "uint256",
          "name": "number",
          "type": "uint256"
        }
      ],
      "name": "txnDone",
      "type": "event"
    },
    {
      "inputs": [],
      "name": "getCount",
      "outputs": [
        {
          "internalType": "uint256",
          "name": "",
          "type": "uint256"
        }
      ],
      "stateMutability": "view",
      "type": "function"
    },
    {
      "inputs": [
        {
          "internalType": "string",
          "name": "sName",
          "type": "string"
        }
      ],
      "name": "setName",
      "outputs": [],
      "stateMutability": "nonpayable",
      "type": "function"
    }
  ]`
