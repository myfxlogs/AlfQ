// Package tradingcore wires trading-core dependencies (adminapi, oms, risksvc)
// into a single process behind a connectRPC h2c mux.
package tradingcore

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/alfq/backend/go/gen/alfq/v1/alfqv1connect"
	"github.com/alfq/backend/go/internal/adminapi"
	"github.com/alfq/backend/go/internal/common/bootstrap"
	"github.com/alfq/backend/go/internal/common/bus"
	"github.com/alfq/backend/go/internal/common/config"
	"github.com/alfq/backend/go/internal/common/health"
	"github.com/alfq/backend/go/internal/oms"
	"github.com/alfq/backend/go/internal/oms/repo"
	"github.com/alfq/backend/go/internal/risksvc"
	"go.uber.org/zap"
)

// RunTradingCore wires all trading-core dependencies and registers routes on mux.
func RunTradingCore(mux *http.ServeMux, d *bootstrap.Deps) error {
	// Health (ConnectRPC)
	path, handler := alfqv1connect.NewHealthServiceHandler(&health.Service{})
	mux.Handle(path, handler)

	// NATS
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}
	busClient, err := bus.Connect(context.Background(), natsURL)
	if err != nil {
		d.Log.Warn("nats connect failed", zap.Error(err))
	}
	if busClient != nil {
		defer busClient.Close()
	}

	// OMS + Risk
	if oms.IsTerminal(0) {
		d.Log.Info("oms state machine loaded")
	}
	_ = repo.NewOrderRepo(d.PG)
	_ = repo.NewPositionRepo(d.PG)

	engine := risksvc.NewEngine()
	kill := &risksvc.KillSwitch{}
	breaker := risksvc.NewBreaker(10)
	_ = breaker
	_ = risksvc.NewKillExecutor()
	_ = risksvc.NewEventRecorder()

	d.Log.Info("risk engine loaded",
		zap.Int("rules", 10),
		zap.Bool("kill_active", kill.IsActive()),
		zap.Bool("breaker_ok", breaker.Allow()),
	)

	// Admin API handlers
	cfg := config.Defaults()
	if cfgPath := os.Getenv("ALFQ_CONFIG"); cfgPath != "" {
		config.Load(cfgPath, cfg)
	}
	svc := adminapi.NewService(d.PG).WithGateways(cfg.MT4Gateway, cfg.MT5Gateway)
	svc.WithLog(d.Log)
	adp := adminapi.NewAdapter(svc)
	mux.Handle(alfqv1connect.NewBrokerServiceHandler(adp))
	mux.Handle(alfqv1connect.NewAccountServiceHandler(adp))
	mux.Handle(alfqv1connect.NewStrategyServiceHandler(adp))
	mux.Handle(alfqv1connect.NewBacktestServiceHandler(adp))
	mux.Handle(alfqv1connect.NewAuditServiceHandler(adp))
	mux.Handle(alfqv1connect.NewSystemSettingsServiceHandler(adp))

	// Auth
	if d.RDB != nil {
		authH, err := adminapi.NewAuthHandler(d.PG, d.RDB)
		if err != nil {
			return fmt.Errorf("auth handler: %w", err)
		}
		authPath, authHandler := alfqv1connect.NewAuthServiceHandler(authH)
		mux.Handle(authPath, authHandler)
		d.Log.Info("auth service registered", zap.String("path", authPath))
	}

	// Override readyz with kill-switch awareness
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if kill.IsActive() {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("kill switch active"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})

	_ = engine
	return nil
}
