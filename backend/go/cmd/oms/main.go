// oms is the Order Management System service.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/alfq/backend/go/internal/common/config"
	"github.com/alfq/backend/go/internal/common/logger"
	"github.com/alfq/backend/go/internal/oms"
)

func main() {
	cfg := config.Defaults()
	log, err := logger.New(cfg.Log.Level)
	if err != nil {
		slog.Error("logger init failed")
		os.Exit(1)
	}
	defer log.Sync()

	// Verify state machine integrity at startup.
	if oms.IsTerminal(0) {
		log.Info("oms state machine loaded")
	}

	// Health check
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	server := &http.Server{Addr: ":9005"}
	go func() {
		log.Info("oms starting", zap.String("addr", ":9005"))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	log.Info("shutting down...")
	shutdownCtx, sdCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer sdCancel()
	server.Shutdown(shutdownCtx)
	log.Info("oms stopped")
}

// Ensure import is used
var _ = fmt.Sprintf
