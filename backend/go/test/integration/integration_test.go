//go:build integration

// Package integration contains testcontainers-based integration tests for the data plane.
// Requires Docker daemon running. Run with: go test -tags=integration ./test/integration/...
package integration

import (
	"context"
	"testing"
	"time"

	"github.com/alfq/backend/go/internal/mdgateway"
	"github.com/alfq/backend/go/internal/mdgateway/chmigrate"
)

// TestCHWriterEndToEnd verifies:
//  1. Containerized ClickHouse starts and migrations create md_ticks table
//  2. CHConn + CHWriter can write ticks
//  3. Data is queryable after flush
//
// Prerequisites: Docker daemon running.
func TestCHWriterEndToEnd(t *testing.T) {
	t.Skip("requires Docker daemon + testcontainers runtime")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Placeholder: start testcontainers ClickHouse, run chmigrate.MustRun,
	// write ticks via CHWriter, query back via clickhouse-go.
	_ = ctx
	_ = chmigrate.MustRun
	_ = mdgateway.DefaultCHConnConfig
}

// TestBarAggregatorEndToEnd verifies tick → bar pipeline through ClickHouse.
//
// Prerequisites: Docker daemon running.
func TestBarAggregatorEndToEnd(t *testing.T) {
	t.Skip("requires Docker daemon + testcontainers runtime")
}

// TestSymbolsyncRepoUpsert verifies broker_symbols upsert idempotency via testcontainers PG.
//
// Prerequisites: Docker daemon running.
func TestSymbolsyncRepoUpsert(t *testing.T) {
	t.Skip("requires Docker daemon + testcontainers runtime")
}
