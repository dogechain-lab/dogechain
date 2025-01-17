package chain

import (
	"math/big"
)

// Params are all the set of params for the chain
type Params struct {
	Forks                *Forks                 `json:"forks"`
	ChainID              int                    `json:"chainID"`
	Engine               map[string]interface{} `json:"engine"`
	BlockGasTarget       uint64                 `json:"blockGasTarget"`
	BlackList            []string               `json:"blackList,omitempty"`
	DDOSProtection       bool                   `json:"ddosProtection,omitempty"`
	DestructiveContracts []string               `json:"destructiveContracts,omitempty"`
}

func (p *Params) GetEngine() string {
	// We know there is already one
	for k := range p.Engine {
		return k
	}

	return ""
}

// Forks specifies when each fork is activated
type Forks struct {
	Homestead      *Fork `json:"homestead,omitempty"`
	Byzantium      *Fork `json:"byzantium,omitempty"`
	Constantinople *Fork `json:"constantinople,omitempty"`
	Petersburg     *Fork `json:"petersburg,omitempty"`
	Istanbul       *Fork `json:"istanbul,omitempty"`
	EIP150         *Fork `json:"EIP150,omitempty"`
	EIP158         *Fork `json:"EIP158,omitempty"`
	EIP155         *Fork `json:"EIP155,omitempty"`
	Preportland    *Fork `json:"pre-portland,omitempty"` // test hardfork only in some test networks
	Portland       *Fork `json:"portland,omitempty"`     // bridge hardfork
	Detroit        *Fork `json:"detroit,omitempty"`      // pos hardfork
}

func (f *Forks) on(ff *Fork, block uint64) bool {
	if ff == nil {
		return false
	}

	return ff.On(block)
}

func (f *Forks) active(ff *Fork, block uint64) bool {
	if ff == nil {
		return false
	}

	return ff.Active(block)
}

func (f *Forks) IsHomestead(block uint64) bool {
	return f.active(f.Homestead, block)
}

func (f *Forks) IsByzantium(block uint64) bool {
	return f.active(f.Byzantium, block)
}

func (f *Forks) IsConstantinople(block uint64) bool {
	return f.active(f.Constantinople, block)
}

func (f *Forks) IsPetersburg(block uint64) bool {
	return f.active(f.Petersburg, block)
}

func (f *Forks) IsEIP150(block uint64) bool {
	return f.active(f.EIP150, block)
}

func (f *Forks) IsEIP158(block uint64) bool {
	return f.active(f.EIP158, block)
}

func (f *Forks) IsEIP155(block uint64) bool {
	return f.active(f.EIP155, block)
}

func (f *Forks) IsPortland(block uint64) bool {
	return f.active(f.Portland, block)
}

func (f *Forks) IsDetroit(block uint64) bool {
	return f.active(f.Detroit, block)
}

func (f *Forks) At(block uint64) ForksInTime {
	return ForksInTime{
		Homestead:      f.active(f.Homestead, block),
		Byzantium:      f.active(f.Byzantium, block),
		Constantinople: f.active(f.Constantinople, block),
		Petersburg:     f.active(f.Petersburg, block),
		Istanbul:       f.active(f.Istanbul, block),
		EIP150:         f.active(f.EIP150, block),
		EIP158:         f.active(f.EIP158, block),
		EIP155:         f.active(f.EIP155, block),
		Preportland:    f.active(f.Preportland, block),
		Portland:       f.active(f.Portland, block),
		Detroit:        f.active(f.Detroit, block),
	}
}

func (f *Forks) IsOnPreportland(block uint64) bool {
	return f.on(f.Preportland, block)
}

func (f *Forks) IsOnPortland(block uint64) bool {
	return f.on(f.Portland, block)
}

func (f *Forks) IsOnDetroit(block uint64) bool {
	return f.on(f.Detroit, block)
}

type Fork uint64

func NewFork(n uint64) *Fork {
	f := Fork(n)

	return &f
}

func (f Fork) On(block uint64) bool {
	return block == uint64(f)
}

func (f Fork) Active(block uint64) bool {
	return block >= uint64(f)
}

func (f Fork) Int() *big.Int {
	return big.NewInt(int64(f))
}

type ForksInTime struct {
	Homestead,
	Byzantium,
	Constantinople,
	Petersburg,
	Istanbul,
	EIP150,
	EIP158,
	EIP155,
	Preportland,
	Portland,
	Detroit bool
}

var AllForksEnabled = &Forks{
	Homestead:      NewFork(0),
	EIP150:         NewFork(0),
	EIP155:         NewFork(0),
	EIP158:         NewFork(0),
	Byzantium:      NewFork(0),
	Constantinople: NewFork(0),
	Petersburg:     NewFork(0),
	Istanbul:       NewFork(0),
	Preportland:    NewFork(10000),
	Portland:       NewFork(10222),
	Detroit:        NewFork(40562),
}
