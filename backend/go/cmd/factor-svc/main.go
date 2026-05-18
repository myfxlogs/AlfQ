// factor-svc computes factor values from bar streams.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/alfq/backend/go/internal/common/config"
	"github.com/alfq/backend/go/internal/common/logger"
	"github.com/alfq/backend/go/internal/factorsvc"
)

func main() {
	cfg := config.Defaults()
	log, err := logger.New(cfg.Log.Level)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	fCfg := factorsvc.Config{
		NatsURL: "nats://localhost:4222",
		Factors: []factorsvc.FactorDef{
			{Name: "sma20", Expression: "sma($close, 20)", Symbols: []string{"EURUSD"}},
			{Name: "sma60", Expression: "sma($close, 60)", Symbols: []string{"EURUSD"}},
			{Name: "rsi14", Expression: "rsi(14)", Symbols: []string{"EURUSD"}},
		},
	}

	engine := factorsvc.NewEngine(fCfg)
	sub := factorsvc.NewSubscriber(engine, fCfg.NatsURL, log)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go func() {
		log.Info("factor-svc starting")
		if err := sub.Start(ctx); err != nil {
			log.Error("subscriber error", zap.Error(err))
		}
	}()

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	server := &http.Server{Addr: ":9002"}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	log.Info("shutting down...")
	shutdownCtx, sdCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer sdCancel()
	server.Shutdown(shutdownCtx)
	log.Info("factor-svc stopped")
}
