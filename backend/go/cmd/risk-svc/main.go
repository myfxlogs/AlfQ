// risk-svc is the risk management service.
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
	"github.com/alfq/backend/go/internal/risksvc"
)

func main() {
	cfg := config.Defaults()
	log, err := logger.New(cfg.Log.Level)
	if err != nil {
		slog.Error("logger init failed")
		os.Exit(1)
	}
	defer log.Sync()

	engine := risksvc.NewEngine()
	kill := &risksvc.KillSwitch{}
	breaker := risksvc.NewBreaker(10)

	executor := risksvc.NewKillExecutor()
	recorder := risksvc.NewEventRecorder()

	log.Info("risk-svc starting",
		zap.Int("rules", 10),
		zap.Bool("kill_active", kill.IsActive()),
		zap.Bool("breaker_ok", breaker.Allow()),
	)

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	server := &http.Server{Addr: ":9004"}
	go func() {
		log.Info("risk-svc starting", zap.String("addr", ":9004"))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	log.Info("shutting down...")
	shutdownCtx, sdCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer sdCancel()
	server.Shutdown(shutdownCtx)
	log.Info("risk-svc stopped")

	// Prevent unused warnings
	_ = engine
	_ = executor
	_ = recorder
	_ = fmt.Sprintf
}
