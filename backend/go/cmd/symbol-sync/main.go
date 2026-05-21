// symbol-sync — manual symbol metadata sync CLI.
// Usage:
//
//	./symbol-sync --account <uuid> [--force] [--direct]
//
// Default: borrows md-gateway's MT session via mthub.
// --direct: dials MT gateway directly (legacy fallback).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/alfq/backend/go/internal/common/config"
	"github.com/alfq/backend/go/internal/common/db/pg"
	"github.com/alfq/backend/go/internal/mdgateway/adapter/mtapi"
	"github.com/alfq/backend/go/internal/mthub"
	"github.com/alfq/backend/go/internal/symbolsync"
	"go.uber.org/zap"
)

func main() {
	accountID := flag.String("account", "", "Account UUID to sync symbols for")
	force := flag.Bool("force", false, "Force re-sync even if recently synced")
	direct := flag.Bool("direct", false, "Dial MT gateway directly (legacy)")
	mthubAddr := flag.String("mthub", "localhost:9001", "mthub address (md-gateway internal port)")
	flag.Parse()

	if *accountID == "" {
		fmt.Fprintln(os.Stderr, "usage: symbol-sync --account <uuid> [--force] [--direct]")
		os.Exit(2)
	}

	log, _ := zap.NewProduction()
	defer log.Sync()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Connect to PG
	pgDSN := os.Getenv("PG_DSN")
	if pgDSN == "" {
		pgDSN = "postgres://alfq:alfq_dev@localhost:5432/alfq?sslmode=disable"
	}
	pool, err := pg.Connect(ctx, pgDSN)
	if err != nil {
		log.Fatal("pg connect failed", zap.Error(err))
	}
	defer pool.Close()

	// Lookup account
	var brokerID, platform, login, password, server string
	err = pool.QueryRow(ctx, `
		SELECT broker_id::text, platform, login, password, server
		FROM accounts WHERE id = $1`,
		*accountID,
	).Scan(&brokerID, &platform, &login, &password, &server)
	if err != nil {
		log.Fatal("account not found", zap.String("id", *accountID), zap.Error(err))
	}

	_ = force
	svc := symbolsync.NewService(pool.Pool, log)

	if *direct {
		// Legacy: dial MT gateway directly.
		gwCfg := config.Defaults().MT5Gateway
		if platform == "MT4" {
			gwCfg = config.Defaults().MT4Gateway
		}
		conn, sessionID, err := mtapi.ConnectSession(ctx, gwCfg, platform, login, password, server)
		if err != nil {
			log.Fatal("mt connect failed", zap.Error(err))
		}
		defer mtapi.DisconnectSession(context.Background(), conn, platform, sessionID)

		if err := svc.Sync(ctx, symbolsync.SyncParams{
			BrokerID: brokerID, Platform: platform, SessionID: sessionID, Conn: conn,
		}); err != nil {
			log.Fatal("symbol sync failed", zap.Error(err))
		}
	} else {
		// Default: borrow md-gateway session via mthub.
		client := mthub.NewClient(*mthubAddr)
		if _, err := client.EnsureSession(ctx, *accountID); err != nil {
			log.Warn("mthub ensure session failed (continuing)", zap.Error(err))
		}
		if err := svc.SyncViaMthub(ctx, client, brokerID, platform, *accountID); err != nil {
			log.Fatal("symbol sync via mthub failed", zap.Error(err))
		}
	}

	var total int
	_ = pool.QueryRow(ctx,
		`SELECT count(*) FROM broker_symbols WHERE broker_id = $1`, brokerID,
	).Scan(&total)

	fmt.Printf("symbol sync complete: account=%s broker=%s total=%d\n", *accountID, brokerID, total)
}
