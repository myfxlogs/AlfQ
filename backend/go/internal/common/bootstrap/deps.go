// Package bootstrap provides a shared startup sequence for all ALFQ Go services.
// It centralises config loading, logger init, infrastructure connections (PG/Redis/NATS/CH),
// and the h2c HTTP server with graceful shutdown — so that each cmd/*/main.go stays ≤ 50 LOC.
package bootstrap

import (
	"net/http"

	"github.com/alfq/backend/go/internal/common/db/pg"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Deps holds infrastructure dependencies that services may need.
type Deps struct {
	Cfg        Config
	Log        *zap.Logger
	PG         *pg.Pool
	RDB        redis.UniversalClient
	NATS       interface{ Close() error }
	CH         *pgxpool.Pool // ClickHouse uses pgx protocol compatibility
	Middleware func(http.Handler) http.Handler // optional: set by registrars that create auth
}

// Config is the subset of config needed by bootstrap.
type Config struct {
	Listen  string
	LogJSON bool
}

// Registrar is called after infrastructure is up, before the server starts.
// Register all gRPC/Connect handlers on the mux here.
type Registrar func(mux *ServeMuxAdapter, d *Deps) error
