package jsonrpc

import (
	"fmt"
	"testing"

	"github.com/dogechain-lab/dogechain/versioning"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

func TestWeb3EndpointSha3(t *testing.T) {
	dispatcher := newDispatcher(hclog.NewNullLogger(), NilMetrics(), newMockStore(), 0, 20, 1000, 0, []Namespace{
		NamespaceWeb3,
	})

	resp, err := dispatcher.Handle([]byte(`{
		"method": "web3_sha3",
		"params": ["0x68656c6c6f20776f726c64"]
	}`))
	assert.NoError(t, err)

	var res string

	assert.NoError(t, expectJSONResult(resp, &res))
	assert.Equal(t, "0x47173285a8d7341e5e972fc677286384f802f8ef42a5ec5f03bbfa254cb01fad", res)
}

func TestWeb3EndpointClientVersion(t *testing.T) {
	chainID := uint64(100)

	dispatcher := newDispatcher(
		hclog.NewNullLogger(),
		NilMetrics(),
		newMockStore(),
		chainID,
		20,
		1000,
		0,
		[]Namespace{
			NamespaceWeb3,
		},
	)

	resp, err := dispatcher.Handle([]byte(`{
		"method": "web3_clientVersion",
		"params": []
	}`))
	assert.NoError(t, err)

	var res string

	assert.NoError(t, expectJSONResult(resp, &res))
	assert.Contains(t, res,
		fmt.Sprintf(
			_clientVersionTemplate,
			chainID,
			versioning.Version,
		),
	)
}
