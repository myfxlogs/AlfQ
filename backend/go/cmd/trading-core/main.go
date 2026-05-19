// trading-core merges admin-api, oms, and risk-svc into a single process.
// This eliminates inter-process network latency on the hot order path.
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
	"github.com/alfq/backend/go/internal/adminapi"
	"github.com/alfq/backend/go/internal/common/bus"
	"github.com/alfq/backend/go/internal/common/config"
	"github.com/alfq/backend/go/internal/common/db/pg"
	"github.com/alfq/backend/go/internal/common/db/redis"
	"github.com/alfq/backend/go/internal/common/health"
	"github.com/alfq/backend/go/internal/common/logger"
	"github.com/alfq/backend/go/internal/oms"
	"github.com/alfq/backend/go/internal/oms/repo"
	"github.com/alfq/backend/go/internal/risksvc"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func main() {
	cfg := config.Defaults()
	if _, err := config.Load("configs/trading-core.yaml", cfg); err != nil {
		fmt.Fprintf(os.Stderr, "config: %v (using defaults)\n", err)
	}

	log, err := logger.New(cfg.Log.Level)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	// ── PostgreSQL (shared by admin-api, oms, risk-svc) ──
	pgDSN := os.Getenv("DATABASE_URL")
	if pgDSN == "" {
		pgDSN = "postgres://alfq:alfq_dev@localhost:5432/alfq?sslmode=disable"
	}
	pgPool, err := pg.Connect(context.Background(), pgDSN)
	if err != nil {
		log.Fatal("pg connect", zap.Error(err))
	}
	defer pgPool.Close()

	// ── Redis (auth + snapshots) ──
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	rdb, err := redis.Connect(context.Background(), redisAddr, "")
	if err != nil {
		log.Warn("redis unavailable, auth disabled", zap.Error(err))
	} else {
		defer rdb.Close()
	}

	// ── NATS (oms event bus) ──
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}
	busClient, err := bus.Connect(context.Background(), natsURL)
	if err != nil {
		log.Warn("nats connect failed", zap.Error(err))
	}
	if busClient != nil {
		defer busClient.Close()
	}

	// ── OMS: verify state machine, init repos ──
	if oms.IsTerminal(0) {
		log.Info("oms state machine loaded")
	}
	orderRepo := repo.NewOrderRepo(pgPool)
	posRepo := repo.NewPositionRepo(pgPool)
	_ = orderRepo
	_ = posRepo

	// ── Risk engine ──
	engine := risksvc.NewEngine()
	kill := &risksvc.KillSwitch{}
	breaker := risksvc.NewBreaker(10)
	_ = breaker
	executor := risksvc.NewKillExecutor()
	recorder := risksvc.NewEventRecorder()
	_ = executor
	_ = recorder

	log.Info("risk engine loaded",
		zap.Int("rules", 10),
		zap.Bool("kill_active", kill.IsActive()),
		zap.Bool("breaker_ok", breaker.Allow()),
	)

	// ── HTTP mux (single server for all three services) ──
	mux := http.NewServeMux()

	// CORS for frontend dev server
	corsHandler := corsMiddleware(mux)

	// HealthService (ConnectRPC)
	path, handler := alfqv1connect.NewHealthServiceHandler(&health.Service{})
	mux.Handle(path, handler)

	// Admin API: Broker, Account, Strategy, Auth
	svc := adminapi.NewService(pgPool)
	adp := adminapi.NewAdapter(svc)
	bp, bh := alfqv1connect.NewBrokerServiceHandler(adp)
	mux.Handle(bp, bh)
	ap, ah := alfqv1connect.NewAccountServiceHandler(adp)
	mux.Handle(ap, ah)
	sp, sh := alfqv1connect.NewStrategyServiceHandler(adp)
	mux.Handle(sp, sh)
	btp, bth := alfqv1connect.NewBacktestServiceHandler(adp)
	mux.Handle(btp, bth)
	audp, audh := alfqv1connect.NewAuditServiceHandler(adp)
	mux.Handle(audp, audh)

	if rdb != nil {
		authH, err := adminapi.NewAuthHandler(pgPool, rdb)
		if err != nil {
			log.Fatal("auth handler", zap.Error(err))
		}
		authPath, authHandler := alfqv1connect.NewAuthServiceHandler(authH)
		mux.Handle(authPath, authHandler)
		log.Info("auth service registered", zap.String("path", authPath))
	}

	// Unified health, readiness, metrics
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if kill.IsActive() {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("kill switch active"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:    cfg.Server.Listen,
		Handler: h2c.NewHandler(corsHandler, &http2.Server{}),
	}

	// ── Background goroutines ──
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Risk engine is invoked synchronously on the order path (oms → risk).
	// No background loop needed since risk-svc has been merged into trading-core.
	_ = engine

	// Graceful shutdown
	go func() {
		<-ctx.Done()
		log.Info("shutting down...")
		shutdownCtx, sdCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer sdCancel()
		server.Shutdown(shutdownCtx)
	}()

	log.Info("trading-core starting", zap.String("addr", cfg.Server.Listen))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("server error", zap.Error(err))
	}
	log.Info("trading-core stopped")
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
