package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	log, err := zap.NewDevelopment() // console-friendly, immediate flush for docker logs
	if err != nil {
		return fmt.Errorf("bootstrap: logger: %w", err)
	}
	defer func() { _ = log.Sync() }() //nolint:errcheck

	// ── Infrastructure connections ──
	d := &Deps{Log: log}

	if !cfg.skipPG {
		dsn := os.Getenv("DATABASE_URL")
		if dsn == "" {
			return fmt.Errorf("bootstrap: DATABASE_URL must be set (no hard-coded credentials allowed)")
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
		redisPassword := os.Getenv("REDIS_PASSWORD")
		rdb := redis.NewClient(&redis.Options{Addr: addr, Password: redisPassword})
		if _, err := rdb.Ping(context.Background()).Result(); err != nil {
			log.Warn("redis unavailable, auth disabled", zap.Error(err))
		} else {
			d.RDB = rdb
			defer func() { _ = rdb.Close() }() //nolint:errcheck
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
	var handler http.Handler = mux
	if d.Middleware != nil {
		handler = d.Middleware(mux)
	}
	srv := newServer(addr, handler)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Shutdown handler in background
	go func() {
		<-ctx.Done()
		log.Info("shutting down...")

		// Run registered shutdown hooks in reverse order
		for i := len(adapter.OnShutdown) - 1; i >= 0; i-- {
			adapter.OnShutdown[i]()
		}

		shutdownWithTimeout(srv, 5*time.Second)
	}()

	// Start server (blocking)
	log.Info(svcName+" starting", zap.String("addr", addr))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server: %w", err)
	}
	log.Info(svcName + " stopped")
	return nil
}
