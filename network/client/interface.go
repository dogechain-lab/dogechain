package client

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

type GrpcClientCloser interface {
	io.Closer

	IsClose() bool
}

func setFinalizerClosedClient(logger hclog.Logger, clt GrpcClientCloser) {
	// print a error log if the client is not closed before GC
	runtime.SetFinalizer(clt, func(c GrpcClientCloser) {
		if !c.IsClose() {
			logger.Error(errClosedClientInFinalizer)
			c.Close()
		}

		runtime.SetFinalizer(c, nil)
	})
}
