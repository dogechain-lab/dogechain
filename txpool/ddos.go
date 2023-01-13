package txpool

import (
	"github.com/dogechain-lab/dogechain/types"
)

// IsDDOSTx returns whether a contract transaction marks as ddos attack
func (p *TxPool) IsDDOSTx(tx *types.Transaction) bool {
	if !p.ddosProtection || tx.To == nil {
		return false
	}

	// skip white list
	if p.isDDOSWhiteList(*tx.To) {
		return false
	}

	// black list contract
	return p.isDDOSContract(*tx.To)
}

func (p *TxPool) isDDOSWhiteList(addr types.Address) bool {
	if _, ok := p.ddosWhiteList.Load(addr); !ok {
		return false
	}

	return true
}

func (p *TxPool) isDDOSContract(addr types.Address) bool {
	v, exists := p.ddosContracts.Load(addr)
	count, _ := v.(int)

	if exists && isCountExceedDDOSLimit(count) {
		return true
	}

	return false
}

func isCountExceedDDOSLimit(count int) bool {
	return count > _ddosThreshold
}

// MarkDDOSTx marks resource consuming transaction as a might-be attack
func (p *TxPool) MarkDDOSTx(tx *types.Transaction) {
	if !p.ddosProtection || tx.To == nil {
		return
	}

	contract := *tx.To
	// update its ddos count
	v, _ := p.ddosContracts.Load(contract)
	count, _ := v.(int)
	count++
	p.ddosContracts.Store(contract, count)

	p.logger.Debug("increase ddos contract transaction count",
		"address", contract,
		"count", count,
	)
}

// reduceDDOSCounts reduces might-be misunderstanding of ddos attack
func (p *TxPool) reduceDDOSCounts() {
	p.ddosContracts.Range(func(key, value interface{}) bool {
		count, _ := value.(int)
		if count <= 0 {
			return true
		}

		count -= _ddosReduceCount
		if count < 0 {
			count = 0
		}

		p.ddosContracts.Store(key, count)

		p.logger.Debug("decrease ddos contract transaction count",
			"address", key,
			"count", count,
		)

		return true
	})
}

// grpc calling

func (p *TxPool) AddWhitelistContracts(contracts []string) (count int) {
	for _, contract := range contracts {
		addr := types.StringToAddress(contract)

		if _, loaded := p.ddosWhiteList.LoadOrStore(addr, 1); !loaded {
			count++
		}
	}

	return count
}

func (p *TxPool) DeleteWhitelistContracts(contracts []string) (count int) {
	for _, contract := range contracts {
		addr := types.StringToAddress(contract)

		if _, loaded := p.ddosWhiteList.LoadAndDelete(addr); loaded {
			count++
		}
	}

	return count
}

const (
	DDosWhiteList = "whitelist"
	DDosBlackList = "blacklist"
)

// GetDDosContractList shows current white list and black list contracts
func (p *TxPool) GetDDosContractList() map[string]map[types.Address]int {
	var (
		ret    = make(map[string]map[types.Address]int, 2)
		blacks = make(map[types.Address]int)
		whites = make(map[types.Address]int)
	)

	p.ddosContracts.Range(func(key, value interface{}) bool {
		addr, _ := key.(types.Address)
		count, _ := value.(int)

		if isCountExceedDDOSLimit(count) {
			blacks[addr] = count
		}

		return true
	})

	p.ddosWhiteList.Range(func(key, value interface{}) bool {
		addr, _ := key.(types.Address)
		whites[addr] = 1

		return true
	})

	ret[DDosBlackList] = blacks
	ret[DDosWhiteList] = whites

	return ret
}
