package tracer

import (
	"testing"
)

// // callTrace is the result of a callTracer run.
// type callTrace struct {
// 	Type    string         `json:"type"`
// 	From    types.Address  `json:"from"`
// 	To      *types.Address `json:"to"`
// 	Input   []byte         `json:"input"`
// 	Output  []byte         `json:"output"`
// 	Gas     uint64         `json:"gas"`
// 	GasUsed uint64         `json:"gasUsed,omitempty"`
// 	Value   *big.Int       `json:"value,omitempty"`
// 	Error   string         `json:"error,omitempty"`
// 	Calls   []callTrace    `json:"calls,omitempty"`
// }

func BenchmarkTransactionTrace(b *testing.B) {
	// key, _ := crypto.BytesToPrivateKey([]byte("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291"))
	// from := crypto.PubKeyToAddress(&key.PublicKey)
	// gas := uint64(1000000) // 1M gas
	// to := types.StringToAddress("0x00000000000000000000000000000000deadbeef")
	// signer := crypto.NewEIP155Signer(100)

	// tx, err := signer.SignTx(
	// 	&types.Transaction{
	// 		Nonce:    1,
	// 		GasPrice: big.NewInt(500),
	// 		Gas:      gas,
	// 		To:       &to,
	// 	},
	// 	key,
	// )
	// if err != nil {
	// 	b.Fatal(err)
	// }

	// txContext := vm.TxContext{
	// 	Origin:   from,
	// 	GasPrice: tx.GasPrice,
	// }
	// context := vm.BlockContext{
	// 	CanTransfer: core.CanTransfer,
	// 	Transfer:    core.Transfer,
	// 	Coinbase:    types.Address{},
	// 	BlockNumber: new(big.Int).SetUint64(uint64(5)),
	// 	Time:        new(big.Int).SetUint64(uint64(5)),
	// 	Difficulty:  big.NewInt(0xffffffff),
	// 	GasLimit:    gas,
	// 	BaseFee:     big.NewInt(8),
	// }
	// alloc := core.GenesisAlloc{}
	// // The code pushes 'deadbeef' into memory, then the other params, and calls CREATE2, then returns
	// // the address
	// loop := []byte{
	// 	byte(vm.JUMPDEST), //  [ count ]
	// 	byte(vm.PUSH1), 0, // jumpdestination
	// 	byte(vm.JUMP),
	// }
	// alloc[types.StringToAddress("0x00000000000000000000000000000000deadbeef")] = core.GenesisAccount{
	// 	Nonce:   1,
	// 	Code:    loop,
	// 	Balance: big.NewInt(1),
	// }
	// alloc[from] = core.GenesisAccount{
	// 	Nonce:   1,
	// 	Code:    []byte{},
	// 	Balance: big.NewInt(500000000000000),
	// }
	// _, statedb := tests.MakePreState(rawdb.NewMemoryDatabase(), alloc, false)
	// // Create the tracer, the EVM environment and run it
	// tracer := logger.NewStructLogger(&logger.Config{
	// 	Debug: false,
	// 	//DisableStorage: true,
	// 	//EnableMemory: false,
	// 	//EnableReturnData: false,
	// })
	// evm := vm.NewEVM(context, txContext, statedb, params.AllEthashProtocolChanges,
	// vm.Config{Debug: true, Tracer: tracer})
	// msg, err := tx.AsMessage(signer, nil)
	// if err != nil {
	// 	b.Fatalf("failed to prepare transaction for tracing: %v", err)
	// }
	// b.ResetTimer()
	// b.ReportAllocs()

	// for i := 0; i < b.N; i++ {
	// 	snap := statedb.Snapshot()
	// 	st := core.NewStateTransition(evm, msg, new(core.GasPool).AddGas(tx.Gas()))
	// 	_, err = st.TransitionDb()
	// 	if err != nil {
	// 		b.Fatal(err)
	// 	}
	// 	statedb.RevertToSnapshot(snap)
	// 	if have, want := len(tracer.StructLogs()), 244752; have != want {
	// 		b.Fatalf("trace wrong, want %d steps, have %d", want, have)
	// 	}
	// 	tracer.Reset()
	// }
}
