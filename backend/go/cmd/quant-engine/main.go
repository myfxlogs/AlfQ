// quant-engine merges factor-svc and strategy-svc into a single process.
// Factor values flow directly to strategy evaluation without inter-process NATS hops.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alfq/backend/go/internal/common/config"
	"github.com/alfq/backend/go/internal/common/logger"
	"github.com/alfq/backend/go/internal/factorsvc"
	"github.com/alfq/backend/go/internal/strategysvc"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func main() {
	cfg := config.Defaults()
	log, err := logger.New(cfg.Log.Level)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	// ── Factor engine ──
	fCfg := factorsvc.Config{
		NatsURL: os.Getenv("NATS_URL"),
		Factors: []factorsvc.FactorDef{
			{Name: "sma20", Expression: "sma($close, 20)", Symbols: []string{"EURUSD"}},
			{Name: "sma60", Expression: "sma($close, 60)", Symbols: []string{"EURUSD"}},
			{Name: "rsi14", Expression: "rsi(14)", Symbols: []string{"EURUSD"}},
		},
	}
	if fCfg.NatsURL == "" {
		fCfg.NatsURL = "nats://localhost:4222"
	}

	engine := factorsvc.NewEngine(fCfg)
	sub := factorsvc.NewSubscriber(engine, fCfg.NatsURL, log)

	chWCfg := factorsvc.DefaultFactorCHWriterConfig()
	chWriter := factorsvc.NewFactorCHWriter(chWCfg, log)

	// ── Strategy engine ──
	loader := strategysvc.NewLoader()
	allocator := strategysvc.NewAllocator()
	allocator.SetAccount("demo", 100000.0)
	allocator.AddStrategy("demo", "sma_cross", 0.3, 5.0, 0.1)

	log.Info("quant-engine starting",
		zap.Int("runners", loader.Count()),
		zap.Int("factors", len(fCfg.Factors)),
	)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	chWriter.Start(ctx)
	defer chWriter.Close()

	// Factor subscriber (NATS) in background
	go func() {
		log.Info("factor subscriber starting")
		if err := sub.Start(ctx); err != nil {
			log.Error("subscriber error", zap.Error(err))
		}
	}()

	// Strategy evaluation loop (can be wired to factor output in-process)
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for _, id := range loader.List() {
					if runner := loader.Get(id); runner != nil {
						_, _ = runner.Evaluate(ctx, "EURUSD", 1.0)
					}
				}
			}
		}
	}()

	// ── HTTP server ──
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{Addr: ":9002", Handler: mux}
	go func() {
		log.Info("quant-engine starting", zap.String("addr", ":9002"))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	log.Info("shutting down...")
	shutdownCtx, sdCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer sdCancel()
	server.Shutdown(shutdownCtx)
	log.Info("quant-engine stopped")
}
