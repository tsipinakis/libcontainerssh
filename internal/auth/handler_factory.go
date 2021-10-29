package auth

import (
	goHttp "net/http"

	"github.com/containerssh/libcontainerssh/http"
	"github.com/containerssh/libcontainerssh/log"
)

// NewHandler creates a handler that is compatible with the Go HTTP server.
func NewHandler(h Handler, logger log.Logger) goHttp.Handler {
	return &handler{
		authzHandler: http.NewServerHandler(&authzHandler{
			backend: h,
			logger:  logger,
		}, logger),
		passwordHandler: http.NewServerHandler(&passwordHandler{
			backend: h,
			logger:  logger,
		}, logger),
		pubkeyHandler: http.NewServerHandler(&pubKeyHandler{
			backend: h,
			logger:  logger,
		}, logger),
	}
}
