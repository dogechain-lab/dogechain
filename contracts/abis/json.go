package abis

const StakingJSONABI = `[
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
                "name": "account",
                "type": "address"
            },
            {
                "indexed": true,
                "internalType": "uint256",
                "name": "amount",
                "type": "uint256"
            }
        ],
        "name": "Staked",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs":
        [
            {
                "indexed": true,
                "internalType": "address",
                "name": "account",
                "type": "address"
            },
            {
                "indexed": true,
                "internalType": "uint256",
                "name": "amount",
                "type": "uint256"
            }
        ],
        "name": "Unstaked",
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
                "name": "account",
                "type": "address"
            }
        ],
        "name": "accountStake",
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
        "name": "governor",
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
                "name": "account",
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
                "name": "gov",
                "type": "address"
            }
        ],
        "name": "setGovernorToken",
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
                "name": "minimum",
                "type": "uint256"
            }
        ],
        "name": "setMinimum",
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
                "name": "threshold",
                "type": "uint256"
            }
        ],
        "name": "setThreshold",
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
        "name": "setValidator",
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
        "name": "stake",
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
        [],
        "name": "unstake",
        "outputs":
        [],
        "stateMutability": "nonpayable",
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
