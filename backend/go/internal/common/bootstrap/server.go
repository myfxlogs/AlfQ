package bootstrap

import (
	"context"
	"net/http"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// ServeMuxAdapter wraps *http.ServeMux so that registrars can mount handlers.
type ServeMuxAdapter struct {
	Mux        *http.ServeMux
	OnShutdown []func() // called in LIFO order during graceful shutdown
}

// newServer creates an h2c server with graceful shutdown.
func newServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:    addr,
		Handler: h2c.NewHandler(handler, &http2.Server{}),
	}
}

// shutdownWithTimeout attempts graceful shutdown with a deadline.
func shutdownWithTimeout(srv *http.Server, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_ = srv.Shutdown(ctx) //nolint:errcheck
}
