package wrappers

import (
	"errors"
	"io"
	"runtime"

	"github.com/hashicorp/go-hclog"
)

var (
	ErrClientClosed = errors.New("client is closed")
)

const (
	errClosedClientInFinalizer = "closing client connection in finalizer"
)

type GrpcClientWrapper interface {
	io.Closer

	IsClose() bool
}

func setFinalizerClosedClient(logger hclog.Logger, clt GrpcClientWrapper) {
	// print a error log if the client is not closed before GC
	runtime.SetFinalizer(clt, func(c GrpcClientWrapper) {
		if !c.IsClose() {
			logger.Error(errClosedClientInFinalizer)
			c.Close()
		}

		runtime.SetFinalizer(c, nil)
	})
}
