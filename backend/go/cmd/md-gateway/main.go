// md-gateway is the ALFQ market data gateway.
// It connects to broker mtapi endpoints and publishes normalized ticks to NATS.
package main

import (
	"context"
	"go.uber.org/zap"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alfq/backend/go/internal/common/config"
	"github.com/alfq/backend/go/internal/common/logger"
	"github.com/alfq/backend/go/internal/mdgateway"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

func main() {
	cfg := config.Defaults()
	log, err := logger.New(cfg.Log.Level)
	if err != nil {
		zap.L().Error("logger init failed", zap.Any("error", err))
		os.Exit(1)
	}
	defer log.Sync()

	// Temporary: hard-coded demo config until Viper YAML is wired.
	// In production, this is loaded from configs/md-gateway.yaml.
	mgrCfg := mdgateway.Config{
		Log: mdgateway.LogConfig{Level: cfg.Log.Level},
		Accounts: []mdgateway.AccountEntry{
			{
				TenantID: "demo",
				Broker:   "demo-broker",
				Platform: "mt5",
				Login:    "123456",
				Password: "demo-pass",
				Server:   "DemoServer",
				Host:     "localhost",
				Port:     "443",
				Symbols:  []string{"EURUSD", "GBPUSD"},
			},
		},
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	manager := mdgateway.NewManager(mgrCfg)
	publisher := mdgateway.NewPublisher(log, "nats://localhost:4222")

	chCfg := mdgateway.DefaultCHWriterConfig()
	chWriter, err := mdgateway.NewCHWriter(chCfg, log)
	if err != nil {
		zap.L().Error("chwriter init failed", zap.Error(err))
		os.Exit(1)
	}
	chWriter.Start(ctx)
	defer chWriter.Close()

	// Connect all gateways and start subscriptions.
	for key, gw := range manager.Connections() {
		if err := gw.Connect(ctx); err != nil {
			log.Error("connect failed", zap.String("key", key), zap.Any("error", err))
			continue
		}
		log.Info("broker connected", zap.String("key", key), zap.String("platform", gw.Platform()))

		handler := func(tick *pb.Tick) {
			if err := publisher.Publish(ctx, tick); err != nil {
				log.Warn("publish failed", zap.Any("error", err))
			}
		}

		go func(gw mdgateway.Gateway) {
			if err := gw.Subscribe(ctx, []string{"EURUSD"}, handler); err != nil {
				log.Error("subscribe failed", zap.Any("error", err))
			}
		}(gw)
	}

	// Health check HTTP server.
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	server := &http.Server{Addr: ":9001", Handler: nil}
	go func() {
		log.Info("md-gateway starting", zap.String("addr", ":9001"))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", zap.Any("error", err))
		}
	}()

	<-ctx.Done()
	log.Info("shutting down...")

	shutdownCtx, sdCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer sdCancel()

	server.Shutdown(shutdownCtx)
	publisher.Close()
	for _, gw := range manager.Connections() {
		gw.Disconnect(shutdownCtx)
	}

	log.Info("md-gateway stopped")
}
