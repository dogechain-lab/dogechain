package common

import (
	"fmt"
	"math/rand"
	"net"
	"testing"

	ma "github.com/multiformats/go-multiaddr"

	"github.com/stretchr/testify/assert"
)

func TestParseMultiaddrIP(t *testing.T) {
	t.Parallel()

	type multiAddrs struct {
		MultiAddr ma.Multiaddr
		IP        net.IP
	}

	ipv4Tests := func() []multiAddrs {
		mas := []multiAddrs{}

		for i := 0; i < 1024; i++ {
			buf := make([]byte, 4)
			for i := 0; i < 4; i++ {
				buf[i] = byte(rand.Intn(256))
			}

			ip := net.IP(buf)

			addr, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/90", ip.String()))
			assert.NoError(t, err)

			mas = append(mas, multiAddrs{
				MultiAddr: addr,
				IP:        ip,
			})
		}

		return mas
	}()

	ipv6Tests := func() []multiAddrs {
		mas := []multiAddrs{}

		for i := 0; i < 1024; i++ {
			buf := make([]byte, 16)
			for i := 0; i < 16; i++ {
				buf[i] = byte(rand.Intn(256))
			}

			ip := net.IP(buf)

			addr, err := ma.NewMultiaddr(fmt.Sprintf("/ip6/%s/tcp/90", ip.String()))
			assert.NoError(t, err)

			mas = append(mas, multiAddrs{
				MultiAddr: addr,
				IP:        ip.To16(),
			})
		}

		return mas
	}()

	t.Run("ipv4", func(t *testing.T) {
		t.Parallel()

		for _, test := range ipv4Tests {
			ip, err := ParseMultiaddrIP(test.MultiAddr)
			assert.NoError(t, err)
			assert.True(t, ip.Equal(test.IP))
		}
	})

	t.Run("ipv6", func(t *testing.T) {
		t.Parallel()

		for _, test := range ipv6Tests {
			ip, err := ParseMultiaddrIP(test.MultiAddr)
			assert.NoError(t, err)
			assert.True(t, ip.Equal(test.IP))
		}
	})
}
