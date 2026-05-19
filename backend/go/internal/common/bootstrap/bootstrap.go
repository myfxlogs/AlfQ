package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"
	"os/signal"
	"syscall"

	"github.com/alfq/backend/go/internal/common/db/pg"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Run orchestrates the full service lifecycle: infrastructure → register → serve → shutdown.
// The register callback receives an *http.ServeMux (via ServeMuxAdapter) for mounting handlers.
func Run(svcName string, register Registrar, opts ...Option) error {
	cfg := runCfg{}
	for _, o := range opts {
		o(&cfg)
	}

	log, err := zap.NewProduction()
	if err != nil {
		return fmt.Errorf("bootstrap: logger: %w", err)
	}
	defer log.Sync()

	// ── Infrastructure connections ──
	d := &Deps{Log: log}

	if !cfg.skipPG {
		dsn := os.Getenv("DATABASE_URL")
		if dsn == "" {
			dsn = "postgres://alfq:alfq_dev@localhost:5432/alfq?sslmode=disable"
		}
		pool, err := pg.Connect(context.Background(), dsn)
		if err != nil {
			log.Warn("pg unavailable", zap.Error(err))
		} else {
			d.PG = pool
			defer pool.Close()
		}
	}

	if !cfg.skipRDB {
		addr := os.Getenv("REDIS_ADDR")
		if addr == "" {
			addr = "localhost:6379"
		}
		rdb := redis.NewClient(&redis.Options{Addr: addr})
		if _, err := rdb.Ping(context.Background()).Result(); err != nil {
			log.Warn("redis unavailable, auth disabled", zap.Error(err))
		} else {
			d.RDB = rdb
			defer rdb.Close()
		}
	}

	// NATS and ClickHouse are optional; services that need them can still connect
	// manually inside register. This keeps bootstrap generic.

	// ── HTTP mux ──
	mux := http.NewServeMux()
	registerHealthEndpoints(mux)

	adapter := &ServeMuxAdapter{Mux: mux}
	if err := register(adapter, d); err != nil {
		return fmt.Errorf("bootstrap: register: %w", err)
	}

	// ── Server ──
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":9000"
	}
	srv := newServer(addr, mux)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Start server in background
	go func() {
		log.Info(svcName+" starting", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	log.Info("shutting down...")
	shutdownWithTimeout(srv, 5*time.Second)
	log.Info(svcName + " stopped")
	return nil
}