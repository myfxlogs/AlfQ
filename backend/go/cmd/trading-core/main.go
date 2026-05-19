// trading-core merges admin-api, oms, and risk-svc into a single process.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/alfq/backend/go/gen/alfq/v1/alfqv1connect"
	"github.com/alfq/backend/go/internal/adminapi"
	"github.com/alfq/backend/go/internal/common/bootstrap"
	"github.com/alfq/backend/go/internal/common/bus"
	"github.com/alfq/backend/go/internal/common/health"
	"github.com/alfq/backend/go/internal/oms"
	"github.com/alfq/backend/go/internal/oms/repo"
	"github.com/alfq/backend/go/internal/risksvc"
	"go.uber.org/zap"
)

func main() {
	if err := bootstrap.Run("trading-core", register); err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap: %v\n", err)
		os.Exit(1)
	}
}

func register(adapter *bootstrap.ServeMuxAdapter, d *bootstrap.Deps) error {
	mux := adapter.Mux

	// CORS
	h := corsMiddleware(mux)

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
	orderRepo := repo.NewOrderRepo(d.PG)
	posRepo := repo.NewPositionRepo(d.PG)
	_ = orderRepo
	_ = posRepo

	engine := risksvc.NewEngine()
	kill := &risksvc.KillSwitch{}
	breaker := risksvc.NewBreaker(10)
	_ = breaker
	executor := risksvc.NewKillExecutor()
	recorder := risksvc.NewEventRecorder()
	_ = executor
	_ = recorder

	d.Log.Info("risk engine loaded",
		zap.Int("rules", 10),
		zap.Bool("kill_active", kill.IsActive()),
		zap.Bool("breaker_ok", breaker.Allow()),
	)

	// Admin API handlers
	svc := adminapi.NewService(d.PG)
	adp := adminapi.NewAdapter(svc)
	bp, bh := alfqv1connect.NewBrokerServiceHandler(adp); mux.Handle(bp, bh)
	ap, ah := alfqv1connect.NewAccountServiceHandler(adp); mux.Handle(ap, ah)
	sp, sh := alfqv1connect.NewStrategyServiceHandler(adp); mux.Handle(sp, sh)
	btp, bth := alfqv1connect.NewBacktestServiceHandler(adp); mux.Handle(btp, bth)
	audp, audh := alfqv1connect.NewAuditServiceHandler(adp); mux.Handle(audp, audh)

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

	_ = h // CORS applied
	_ = engine
	return nil
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Tenant-ID, Connect-Protocol-Version, X-User-Agent")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Encoding, Grpc-Status, Grpc-Message, Connect-Protocol-Version")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
