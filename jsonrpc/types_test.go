package jsonrpc

import (
	"bytes"
	"encoding/json"
	"math/big"
	"reflect"
	"strings"
	"testing"
	"text/template"

	"github.com/dogechain-lab/dogechain/helper/hex"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/stretchr/testify/assert"
)

func TestBasicTypes_Encode(t *testing.T) {
	// decode basic types
	cases := []struct {
		obj interface{}
		dec interface{}
		res string
	}{
		{
			argBig(*big.NewInt(10)),
			&argBig{},
			"0xa",
		},
		{
			argUint64(10),
			argUintPtr(0),
			"0xa",
		},
		{
			argBytes([]byte{0x1, 0x2}),
			&argBytes{},
			"0x0102",
		},
	}

	for _, c := range cases {
		res, err := json.Marshal(c.obj)
		assert.NoError(t, err)
		assert.Equal(t, strings.Trim(string(res), "\""), c.res)
		assert.NoError(t, json.Unmarshal(res, c.dec))
	}
}

func TestDecode_TxnArgs(t *testing.T) {
	var (
		addr = types.Address{0x0}
		num  = argUint64(16)
		hex  = argBytes([]byte{0x01})
	)

	cases := []struct {
		data string
		res  *txnArgs
	}{
		{
			data: `{
				"to": "{{.Libp2pAddr}}",
				"gas": "0x10",
				"data": "0x01",
				"value": "0x01"
			}`,
			res: &txnArgs{
				To:    &addr,
				Gas:   &num,
				Data:  &hex,
				Value: &hex,
			},
		},
	}

	for _, c := range cases {
		tmpl, err := template.New("test").Parse(c.data)
		assert.NoError(t, err)

		config := map[string]string{
			"Libp2pAddr": (types.Address{}).String(),
			"Hash":       (types.Hash{}).String(),
		}

		buffer := new(bytes.Buffer)
		assert.NoError(t, tmpl.Execute(buffer, config))

		r := &txnArgs{}
		assert.NoError(t, json.Unmarshal(buffer.Bytes(), &r))

		if !reflect.DeepEqual(r, c.res) {
			t.Fatal("Resulting data and expected values are not equal")
		}
	}
}

func TestToTransaction_Returns_V_R_S_ValuesWithoutLeading0(t *testing.T) {
	hexWithLeading0 := "0x0ba93811466694b3b3cb8853cb8227b7c9f49db10bf6e7db59d20ac904961565"
	hexWithoutLeading0 := "0xba93811466694b3b3cb8853cb8227b7c9f49db10bf6e7db59d20ac904961565"
	v, _ := hex.DecodeHex(hexWithLeading0)
	r, _ := hex.DecodeHex(hexWithLeading0)
	s, _ := hex.DecodeHex(hexWithLeading0)
	txn := types.Transaction{
		Nonce:    0,
		GasPrice: big.NewInt(0),
		Gas:      0,
		To:       nil,
		Value:    big.NewInt(0),
		Input:    nil,
		V:        new(big.Int).SetBytes(v),
		R:        new(big.Int).SetBytes(r),
		S:        new(big.Int).SetBytes(s),
		From:     types.Address{},
	}

	txn.Hash()

	jsonTx := toTransaction(&txn, nil, nil, nil)

	jsonV, _ := jsonTx.V.MarshalText()
	jsonR, _ := jsonTx.R.MarshalText()
	jsonS, _ := jsonTx.S.MarshalText()

	assert.Equal(t, hexWithoutLeading0, string(jsonV))
	assert.Equal(t, hexWithoutLeading0, string(jsonR))
	assert.Equal(t, hexWithoutLeading0, string(jsonS))
}

func Test_toJSONHeader(t *testing.T) {
	rawHeaderStr := `{
    "parentHash": "0x5b1840d0a559112b42208b074284c92d32ddc876ab36d3a40a6ef109c3230899",
    "sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
    "miner": "0x32fC6Db2B0500fF103A810B46Daa26d35CE5172f",
    "stateRoot": "0x7d29e307e0a2e7c00ec8ab96b39d31496e4a27184818c9b586bf4f0a252c46e0",
    "transactionsRoot": "0x0fef8f013ada9a1b491499a90441a109ffaf28e018c9f8540811468a39e8fdba",
    "receiptsRoot": "0xd55415bfb3eb6f64e8501f4821c38b33284828692d98df442fba7946e7b5e205",
    "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
    "difficulty": "0x75f",
    "number": "0x75f",
    "gasLimit": "0x500000",
    "gasUsed": "0x5701",
    "timestamp": "0x6389b20e",
    "extraData": "0x0000000000000000000000000000000000000000000000000000000000000000f89ed59432fc6db2b0500ff103a810b46daa26d35ce5172fb841d8a49a60cbeb222cf4239f8d14581422b938c0265fdeee8fba36e7506be4e8d859fec6b334391b2394fad8dd319851544ff8915dea92b2c6a2c889a3565a8b3801f843b84165f19bb45cae54537a950c0df109277767c65a8a6c738a46a5a89db3d57bc8f473ddcf0d276728fc935d8b1dc0e3db23521f38c935e924201411404f250e7aa801",
    "mixHash": "0x63746963616c2062797a616e74696e65206661756c7420746f6c6572616e6365",
    "nonce": "0x0000000000000000",
    "hash": "0x85492e41d07c4886706650c1d0754856dc7c92d1dd311fb18f6d425c2dd1f897"
}`

	var bloom types.Bloom
	err := bloom.UnmarshalText([]byte("0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"))

	assert.NoError(t, err)

	expectedHeader := &jsonHeader{
		ParentHash:   types.StringToHash("0x5b1840d0a559112b42208b074284c92d32ddc876ab36d3a40a6ef109c3230899"),
		Sha3Uncles:   types.StringToHash("0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347"),
		Miner:        types.StringToAddress("0x32fC6Db2B0500fF103A810B46Daa26d35CE5172f"),
		StateRoot:    types.StringToHash("0x7d29e307e0a2e7c00ec8ab96b39d31496e4a27184818c9b586bf4f0a252c46e0"),
		TxRoot:       types.StringToHash("0x0fef8f013ada9a1b491499a90441a109ffaf28e018c9f8540811468a39e8fdba"),
		ReceiptsRoot: types.StringToHash("0xd55415bfb3eb6f64e8501f4821c38b33284828692d98df442fba7946e7b5e205"),
		LogsBloom:    bloom,
		Difficulty:   1887,
		Number:       1887,
		GasLimit:     5242880,
		GasUsed:      22273,
		Timestamp:    1669968398,
		ExtraData:    types.StringToBytes("0x0000000000000000000000000000000000000000000000000000000000000000f89ed59432fc6db2b0500ff103a810b46daa26d35ce5172fb841d8a49a60cbeb222cf4239f8d14581422b938c0265fdeee8fba36e7506be4e8d859fec6b334391b2394fad8dd319851544ff8915dea92b2c6a2c889a3565a8b3801f843b84165f19bb45cae54537a950c0df109277767c65a8a6c738a46a5a89db3d57bc8f473ddcf0d276728fc935d8b1dc0e3db23521f38c935e924201411404f250e7aa801"),
		MixHash:      types.StringToHash("0x63746963616c2062797a616e74696e65206661756c7420746f6c6572616e6365"),
		Nonce:        types.Nonce{},
		Hash:         types.StringToHash("0x85492e41d07c4886706650c1d0754856dc7c92d1dd311fb18f6d425c2dd1f897"),
	}

	raw, err := json.Marshal(rawHeaderStr)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	jh := &jsonHeader{}

	err = json.Unmarshal(raw, &jh)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	assert.Equal(t, expectedHeader, jh)
}
