package argtype

import (
	"encoding/json"
	"math/big"
	"strings"
	"testing"

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
			Big(*big.NewInt(10)),
			&Big{},
			"0xa",
		},
		{
			Uint64(10),
			UintPtr(0),
			"0xa",
		},
		{
			Bytes([]byte{0x1, 0x2}),
			&Bytes{},
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
