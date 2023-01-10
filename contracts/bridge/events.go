package bridge

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/dogechain-lab/dogechain/contracts/abis"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/umbracle/ethgo"
)

const (
	EventDeposited = "Deposited"
	EventWithdrawn = "Withdrawn"
	EventBurned    = "Burned"

	fieldReceiver = "receiver"
	fieldAmount   = "amount"
	fieldFee      = "fee"
	fieldSender   = "sender"
)

// Frequently used methods. Must exist.
var (
	BridgeDepositedEvent   = abis.BridgeABI.Events[EventDeposited]
	BridgeDepositedEventID = types.Hash(BridgeDepositedEvent.ID())
	BridgeWithdrawnEvent   = abis.BridgeABI.Events[EventWithdrawn]
	BridgeWithdrawnEventID = types.Hash(BridgeWithdrawnEvent.ID())
	BridgeEventBurnedEvent = abis.BridgeABI.Events[EventBurned]
	BridgeBurnedEventID    = types.Hash(BridgeEventBurnedEvent.ID())
)

type DepositedLog struct {
	Receiver types.Address
	Amount   *big.Int
	Fee      *big.Int
}

func ParseBridgeDepositedLog(log *types.Log) (*DepositedLog, error) {
	topics := make([]ethgo.Hash, 0, len(log.Topics))
	for _, topic := range log.Topics {
		topics = append(topics, ethgo.Hash(topic))
	}

	w3Log, err := BridgeDepositedEvent.ParseLog(&ethgo.Log{
		Address: ethgo.Address(log.Address),
		Topics:  topics,
		Data:    log.Data,
	})
	if err != nil {
		return nil, err
	}

	receiver, ok := w3Log[fieldReceiver]
	if !ok {
		return nil, errors.New("address not exists in Deposited event")
	}

	account, ok := receiver.(ethgo.Address)
	if !ok {
		return nil, errors.New("address downcast failed")
	}

	amount, ok := w3Log[fieldAmount]
	if !ok {
		return nil, errors.New("amount not exists in Deposited event")
	}

	bigAmount, ok := amount.(*big.Int)
	if !ok {
		return nil, errors.New("amount downcast failed")
	}

	return &DepositedLog{
		Receiver: types.Address(account),
		Amount:   bigAmount,
	}, nil
}

type WithdrawnLog struct {
	Contract types.Address
	Amount   *big.Int
	Fee      *big.Int
}

func ParseBridgeWithdrawnLog(log *types.Log) (*WithdrawnLog, error) {
	topics := make([]ethgo.Hash, 0, len(log.Topics))
	for _, topic := range log.Topics {
		topics = append(topics, ethgo.Hash(topic))
	}

	w3Log, err := BridgeWithdrawnEvent.ParseLog(&ethgo.Log{
		Address: ethgo.Address(log.Address),
		Topics:  topics,
		Data:    log.Data,
	})
	if err != nil {
		return nil, err
	}

	amount, err := getBigIntFromWithdrawnLog(w3Log, fieldAmount)
	if err != nil {
		return nil, err
	}

	fee, err := getBigIntFromWithdrawnLog(w3Log, fieldFee)
	if err != nil {
		return nil, err
	}

	return &WithdrawnLog{
		Contract: log.Address,
		Amount:   amount,
		Fee:      fee,
	}, nil
}

func getBigIntFromWithdrawnLog(log map[string]interface{}, key string) (*big.Int, error) {
	v, ok := log[key]
	if !ok {
		return nil, fmt.Errorf("%s not exists in Withdrawn event", key)
	}

	bigVal, ok := v.(*big.Int)
	if !ok {
		return nil, fmt.Errorf("%s downcast failed", key)
	}

	return bigVal, nil
}

type BurnedLog struct {
	Sender types.Address
	Amount *big.Int
}

func ParseBridgeBurnedLog(log *types.Log) (*BurnedLog, error) {
	topics := make([]ethgo.Hash, 0, len(log.Topics))
	for _, topic := range log.Topics {
		topics = append(topics, ethgo.Hash(topic))
	}

	w3Log, err := BridgeEventBurnedEvent.ParseLog(&ethgo.Log{
		Address: ethgo.Address(log.Address),
		Topics:  topics,
		Data:    log.Data,
	})
	if err != nil {
		return nil, err
	}

	sender, ok := w3Log[fieldSender]
	if !ok {
		return nil, errors.New("address not exists in Burned event")
	}

	account, ok := sender.(ethgo.Address)
	if !ok {
		return nil, errors.New("address downcast failed")
	}

	amount, err := getBigIntFromWithdrawnLog(w3Log, fieldAmount)
	if err != nil {
		return nil, err
	}

	return &BurnedLog{
		Sender: types.Address(account),
		Amount: amount,
	}, nil
}
