// Package chmigrate provides minimal ClickHouse schema migration.
// SQL files are embedded in the binary.
package chmigrate

import (
	"context"
	"embed"
	"sort"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
	"go.uber.org/zap"
)

//go:embed *.sql
var migrations embed.FS

// Run executes all embedded .sql migrations against the given ClickHouse connection.
// Idempotent: uses CREATE TABLE IF NOT EXISTS.
func Run(ctx context.Context, conn clickhouse.Conn, log *zap.Logger) error {
	entries, err := migrations.ReadDir(".")
	if err != nil {
		log.Warn("chmigrate: read embedded dir failed", zap.Error(err))
		return nil
	}
	if len(entries) == 0 {
		log.Info("chmigrate: no embedded migrations found, skipping")
		return nil
	}

	// Sort so 001 runs before 002 etc.
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		data, err := migrations.ReadFile(e.Name())
		if err != nil {
			log.Warn("chmigrate: read embedded file failed", zap.String("file", e.Name()), zap.Error(err))
			continue
		}
		sql := string(data)
		if strings.TrimSpace(sql) == "" {
			continue
		}
		if err := conn.Exec(ctx, sql); err != nil {
			log.Error("chmigrate: exec failed",
				zap.String("file", e.Name()),
				zap.Error(err),
			)
			return err
		}
		log.Info("chmigrate: applied", zap.String("file", e.Name()))
	}
	return nil
}

// MustRun panics on error.
func MustRun(ctx context.Context, conn clickhouse.Conn, log *zap.Logger) {
	if err := Run(ctx, conn, log); err != nil {
		log.Fatal("chmigrate: fatal", zap.Error(err))
	}
}
