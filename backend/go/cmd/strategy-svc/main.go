// strategy-svc executes strategy deployments and generates trading signals.
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
	"github.com/alfq/backend/go/internal/strategysvc"
)

func main() {
	cfg := config.Defaults()
	log, err := logger.New(cfg.Log.Level)
	if err != nil {
		slog.Error("logger init failed")
		os.Exit(1)
	}
	defer log.Sync()

	loader := strategysvc.NewLoader()
	allocator := strategysvc.NewAllocator()
	allocator.SetAccount("demo", 100000.0)
	allocator.AddStrategy("demo", "sma_cross", 0.3, 5.0, 0.1)

	log.Info("strategy-svc starting",
		zap.Int("runners", loader.Count()),
	)

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	server := &http.Server{Addr: ":9003"}
	go func() {
		log.Info("strategy-svc starting", zap.String("addr", ":9003"))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	log.Info("shutting down...")
	shutdownCtx, sdCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer sdCancel()
	server.Shutdown(shutdownCtx)
	log.Info("strategy-svc stopped")

	_ = loader
	_ = allocator
	_ = fmt.Sprintf
}
