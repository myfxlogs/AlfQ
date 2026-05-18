// admin-api is the minimal HTTP/Connect entrypoint for ALFQ.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alfq/backend/go/gen/alfq/v1/alfqv1connect"
	"github.com/alfq/backend/go/internal/common/config"
	"github.com/alfq/backend/go/internal/common/health"
	"github.com/alfq/backend/go/internal/common/logger"

	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func main() {
	cfg := config.Defaults()

	if _, err := config.Load("configs/admin-api.yaml", cfg); err != nil {
		fmt.Fprintf(os.Stderr, "config: %v (using defaults)\n", err)
	}

	log, err := logger.New(cfg.Log.Level)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	mux := http.NewServeMux()

	path, handler := alfqv1connect.NewHealthServiceHandler(&health.Service{})
	mux.Handle(path, handler)

	server := &http.Server{
		Addr:    cfg.Server.Listen,
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		log.Info("shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	log.Info("admin-api starting", zap.String("addr", cfg.Server.Listen))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("server error", zap.Error(err))
	}
}
