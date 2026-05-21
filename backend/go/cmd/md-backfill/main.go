// md-backfill — historical bar backfill CLI.
// Usage:
//
//	./md-backfill --account <uuid> --symbols EURUSD --periods 1h --from 2024-01-01 --to 2025-01-01
//
// Default: borrows md-gateway's MT session via mthub.
// --direct: dials MT gateway directly (legacy fallback).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/alfq/backend/go/internal/common/config"
	"github.com/alfq/backend/go/internal/mdgateway/backfill"
	"github.com/alfq/backend/go/internal/mthub"
	"go.uber.org/zap"
)

func main() {
	accountID := flag.String("account", "", "Account UUID (mthub path)")
	gateway := flag.String("gateway", os.Getenv("MT5_GATEWAY_ADDR"), "MT5 gateway addr (host:port) — direct path only")
	login := flag.String("login", "", "MT5 account login — direct path only")
	password := flag.String("password", "", "MT5 account password — direct path only")
	server := flag.String("server", "", "MT5 broker server (host:port) — direct path only")
	symbols := flag.String("symbols", "EURUSD", "comma-separated symbols")
	periods := flag.String("periods", "1h,1d", "comma-separated periods")
	from := flag.String("from", "", "start date (YYYY-MM-DD)")
	to := flag.String("to", "", "end date (YYYY-MM-DD)")
	direct := flag.Bool("direct", false, "Dial MT gateway directly (legacy)")
	mthubAddr := flag.String("mthub", "localhost:9001", "mthub address (md-gateway internal port)")
	flag.Parse()

	fromDate, err := time.Parse("2006-01-02", *from)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid --from date: %v\n", err)
		os.Exit(1)
	}
	toDate, err := time.Parse("2006-01-02", *to)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid --to date: %v\n", err)
		os.Exit(1)
	}

	log, _ := zap.NewDevelopment()
	defer log.Sync()

	symList := strings.Split(*symbols, ",")
	perList := strings.Split(*periods, ",")

	if *direct {
		if *gateway == "" || *login == "" || *password == "" || *server == "" {
			fmt.Fprintf(os.Stderr, "Usage (--direct): %s --direct --gateway <addr> --login <N> --password <pw> --server <host:port> --from YYYY-MM-DD --to YYYY-MM-DD\n", os.Args[0])
			os.Exit(1)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		gw := config.GatewayConfig{
			Addr:    *gateway,
			UseTLS:  true,
			Timeout: 30 * time.Second,
		}
		log.Info("connecting to MT5 gateway (direct)", zap.String("addr", *gateway))
		sess, err := backfill.Connect(ctx, gw, *login, *password, *server)
		if err != nil {
			fmt.Fprintf(os.Stderr, "connect failed: %v\n", err)
			os.Exit(1)
		}
		defer sess.Close()

		runBackfill(sess, symList, perList, fromDate, toDate, log)
	} else {
		if *accountID == "" {
			fmt.Fprintf(os.Stderr, "Usage (mthub): %s --account <uuid> --symbols EURUSD --periods 1h --from YYYY-MM-DD --to YYYY-MM-DD\n", os.Args[0])
			os.Exit(1)
		}
		ctx := context.Background()
		client := mthub.NewClient(*mthubAddr)
		if _, err := client.EnsureSession(ctx, *accountID); err != nil {
			log.Warn("mthub ensure session failed (continuing)", zap.Error(err))
		}
		log.Info("backfill via mthub not yet implemented (MH-4 backlog)", zap.String("account", *accountID))
		fmt.Println("mthub backfill path: PriceHistory RPC not yet wired — use --direct for now")
		os.Exit(0)
	}
}

func runBackfill(sess *backfill.Session, symList, perList []string, fromDate, toDate time.Time, log *zap.Logger) {
	total := 0
	for _, sym := range symList {
		sym = strings.TrimSpace(sym)
		if sym == "" {
			continue
		}
		for _, per := range perList {
			per = strings.TrimSpace(per)
			if per == "" {
				continue
			}
			log.Info("fetching", zap.String("symbol", sym), zap.String("period", per))
			bars, err := sess.FetchBars(context.Background(), sym, per, fromDate, toDate, log)
			if err != nil {
				fmt.Fprintf(os.Stderr, "fetch %s/%s: %v\n", sym, per, err)
				continue
			}
			log.Info("fetched", zap.String("symbol", sym), zap.String("period", per), zap.Int("bars", len(bars)))
			total += len(bars)

			for _, b := range bars {
				fmt.Printf(`{"symbol":"%s","period":"%s","time":%d,"open":%.5f,"high":%.5f,"low":%.5f,"close":%.5f,"volume":%.2f}`+"\n",
					sym, per, b.Time, b.Open, b.High, b.Low, b.Close, b.Volume)
			}
		}
	}
	log.Info("done", zap.Int("total_bars", total))
}
