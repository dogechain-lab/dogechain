package e2e

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/dogechain-lab/dogechain/consensus/ibft"
	"github.com/dogechain-lab/dogechain/e2e/framework"
	"github.com/dogechain-lab/dogechain/helper/tests"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/stretchr/testify/assert"
	"github.com/umbracle/go-web3"
)

func TestIbft_Transfer(t *testing.T) {
	senderKey, senderAddr := tests.GenerateKeyAndAddr(t)
	_, receiverAddr := tests.GenerateKeyAndAddr(t)

	ibftManager := framework.NewIBFTServersManager(
		t,
		IBFTMinNodes,
		IBFTDirPrefix,
		false,
		func(i int, config *framework.TestServerConfig) {
			config.Premine(senderAddr, framework.EthToWei(10))
			config.SetSeal(true)
		})

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	ibftManager.StartServers(ctx)

	srv := ibftManager.GetServer(0)

	for i := 0; i < IBFTMinNodes-1; i++ {
		txn := &framework.PreparedTransaction{
			From:     senderAddr,
			To:       &receiverAddr,
			GasPrice: big.NewInt(10000),
			Gas:      1000000,
			Value:    framework.EthToWei(1),
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		receipt, err := srv.SendRawTx(ctx, txn, senderKey)

		assert.NoError(t, err)
		assert.NotNil(t, receipt)
		assert.NotNil(t, receipt.TransactionHash)
	}
}

func TestIbft_TransactionFeeRecipient(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		contractCall bool
		txAmount     *big.Int
		static       bool
	}{
		{
			name:         "transfer transaction",
			contractCall: false,
			txAmount:     framework.EthToWei(1),
			static:       false,
		},
		{
			name:         "contract function execution",
			contractCall: true,
			txAmount:     big.NewInt(0),
			static:       false,
		},
		{
			name:         "transfer transaction (static node)",
			contractCall: false,
			txAmount:     framework.EthToWei(1),
			static:       true,
		},
		{
			name:         "contract function execution (static node)",
			contractCall: true,
			txAmount:     big.NewInt(0),
			static:       true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			senderKey, senderAddr := tests.GenerateKeyAndAddr(t)
			_, receiverAddr := tests.GenerateKeyAndAddr(t)

			ibftManager := framework.NewIBFTServersManager(
				t,
				IBFTMinNodes,
				IBFTDirPrefix,
				tc.static,
				func(i int, config *framework.TestServerConfig) {
					config.Premine(senderAddr, framework.EthToWei(10))
					config.SetSeal(true)
					config.SetBlockTime(1)
				})

			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			ibftManager.StartServers(ctx)

			srv := ibftManager.GetServer(0)
			clt := srv.JSONRPC()

			{
				// wait for the first block
				_, err := tests.RetryUntilTimeout(ctx, func() (interface{}, bool) {
					num, err := srv.GetLatestBlockHeight()
					if err != nil {
						return nil, true
					}
					if num <= 1 {
						return nil, true
					}

					return num, false
				})

				assert.NoError(t, err)
			}

			txn := &framework.PreparedTransaction{
				From:     senderAddr,
				To:       &receiverAddr,
				GasPrice: big.NewInt(10000),
				Gas:      1000000,
				Value:    tc.txAmount,
			}

			if tc.contractCall {
				// Deploy contract
				deployTx := &framework.PreparedTransaction{
					From:     senderAddr,
					GasPrice: big.NewInt(10),
					Gas:      1000000,
					Value:    big.NewInt(0),
					Input:    framework.MethodSig("setA1"),
				}
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				receipt, err := srv.SendRawTx(ctx, deployTx, senderKey)
				assert.NoError(t, err)
				assert.NotNil(t, receipt)

				contractAddr := types.Address(receipt.ContractAddress)
				txn.To = &contractAddr
				txn.Input = framework.MethodSig("setA1")
			}

			ctx1, cancel1 := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel1()
			receipt, err := srv.SendRawTx(ctx1, txn, senderKey)
			assert.NoError(t, err)
			assert.NotNil(t, receipt)

			// Get the block proposer from the extra data seal
			assert.NotNil(t, receipt.BlockHash)
			block, err := clt.Eth().GetBlockByHash(receipt.BlockHash, false)
			assert.NoError(t, err)
			extraData := &ibft.IstanbulExtra{}
			extraDataWithoutVanity := block.ExtraData[ibft.IstanbulExtraVanity:]
			err = extraData.UnmarshalRLP(extraDataWithoutVanity)
			assert.NoError(t, err)

			proposerAddr, err := framework.EcrecoverFromBlockhash(types.Hash(block.Hash), extraData.Seal)
			assert.NoError(t, err)

			// Given that this is the first transaction on the blockchain, proposer's balance should be equal to the tx fee
			balanceProposer, err := clt.Eth().GetBalance(web3.Address(proposerAddr), web3.Latest)
			assert.NoError(t, err)

			txFee := new(big.Int).Mul(new(big.Int).SetUint64(receipt.GasUsed), txn.GasPrice)
			assert.Equalf(t, txFee, balanceProposer, "Proposer didn't get appropriate transaction fee")
		})
	}
}
