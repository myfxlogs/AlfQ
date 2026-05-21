// Package mdgateway — gateway runner that wires connections and starts the heartbeat loop.
package mdgateway

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/alfq/backend/go/internal/common/bootstrap"
	"github.com/alfq/backend/go/internal/common/db/redis"
	"github.com/alfq/backend/go/internal/mdgateway/chmigrate"
	"github.com/alfq/backend/go/internal/mthub"
	"github.com/alfq/backend/go/internal/symbolsync"
	mthubv1connect "github.com/alfq/backend/go/gen/alfq/mthub/v1/mthubv1connect"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/jackc/pgx/v5/pgxpool"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

// RunGateway wires all gateway connections, starts the heartbeat, and registers /readyz.
// Loads active accounts from PG (no static config).
func RunGateway(mux *http.ServeMux, d *bootstrap.Deps, natsURL, redisAddr string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if d.RDB == nil {
		redisClient, err := redis.Connect(ctx, redisAddr, "")
		if err == nil {
			d.RDB = redisClient
			defer func() { _ = redisClient.Close() }() //nolint:errcheck
		}
	}

	// Canonical resolver: strips broker suffixes (e.g. EURUSD.m → EURUSD)
	normalizer := NewNormalizer(NewMapResolver())

	// Load accounts dynamically from PG
	manager := NewEmptyManager()
	manager.SetNormalizer(normalizer)
	if d.PG != nil {
		accounts, err := loadAccounts(d.PG.Pool)
		if err != nil {
			d.Log.Warn("load accounts failed", zap.Error(err))
		} else {
			for _, a := range accounts {
				manager.AddGateway(a)
			}
			d.Log.Info("accounts loaded from PG", zap.Int("count", len(accounts)))
		}
	}

	// MT Session Hub: exposes internal RPC for trading-core / CLI to borrow sessions.
	lookupGW := func(brokerID string) (mthub.Gateway, bool) {
		for _, g := range manager.Connections() {
			if g.BrokerID() == brokerID {
				return g, true
			}
		}
		return nil, false
	}
	hub := mthub.NewHub(lookupGW, d.Log)
	events := mthub.NewOrderEventBroker()
	mtHubSvc := mthub.NewMtHubService(hub, events, d.Log)
	mux.Handle(mthubv1connect.NewMtHubServiceHandler(mtHubSvc))
	d.Log.Info("mthub registered on http mux")

	publisher := NewPublisher(d.Log, natsURL)
	if err := publisher.Connect(ctx); err != nil {
		d.Log.Warn("nats connect failed", zap.Error(err))
	}

	// ClickHouse connection + migration + writer
	chConnCfg := DefaultCHConnConfig()
	if addr := os.Getenv("CH_ADDR"); addr != "" {
		chConnCfg.Addr = addr
	}
	if pwd := os.Getenv("CH_PASSWORD"); pwd != "" {
		chConnCfg.Password = pwd
	}
	chConn, err := NewCHConn(chConnCfg, d.Log)
	if err != nil {
		return fmt.Errorf("chconn: %w", err)
	}
	defer func() { _ = chConn.Close() }() //nolint:errcheck

	if conn, err := chConn.Conn(ctx); err == nil {
		chmigrate.MustRun(ctx, conn, d.Log)
	}

	// Spill replay: replay any failed writes from previous runs
	spill := NewSpillReplay("/tmp/alfq-ch-spill", chConn, d.Log)
	if count, err := spill.Replay(ctx); err != nil {
		d.Log.Warn("spill replay failed", zap.Error(err))
	} else if count > 0 {
		d.Log.Info("spill replay complete", zap.Int("rows", count))
	}

	chWriter := NewCHWriter(DefaultCHWriterConfig(), chConn, d.Log)
	chWriter.Start(context.Background()) // lives for process lifetime

	// Data quality engine
	qc := NewQuality(DefaultQualityConfig())

	// Bar aggregator: tick → OHLCV bars (1m/5m/15m/1h/4h/1d)
	agg := NewAggregator()
	barTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "md_bar_total", Help: "Total bars emitted",
	}, []string{"broker", "canonical", "period"})
	prometheus.MustRegister(barTotal)

	tickTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "md_tick_total", Help: "Total normalized ticks received",
	}, []string{"broker", "symbol"})
	prometheus.MustRegister(tickTotal)

	connStates := make(map[string]bool)
	for key, gw := range manager.Connections() {
		connStates[key] = false
		if err := gw.Connect(ctx); err != nil {
			d.Log.Error("connect failed", zap.String("key", key), zap.Error(err))
			continue
		}
		connStates[key] = true
		d.Log.Info("broker connected", zap.String("key", key))

		// Trigger async symbol sync to refresh broker_symbols from the MT server
		go func(gw Gateway) {
			syncCtx, syncCancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer syncCancel()
			svc := symbolsync.NewService(d.PG.Pool, d.Log)
			if err := svc.Sync(syncCtx, symbolsync.SyncParams{
				BrokerID:  gw.BrokerID(),
				Platform:  gw.Platform(),
				SessionID: gw.SessionID(),
				Conn:      gw.Conn(),
			}); err != nil {
				d.Log.Warn("symbol sync failed", zap.String("key", key), zap.Error(err))
			}
		}(gw)

		handler := func(key string, gw Gateway, tick *pb.Tick) {
			tickTotal.WithLabelValues(tick.Broker, tick.Symbol).Inc()
			d.Log.Debug("tick received",
				zap.String("broker", tick.Broker),
				zap.String("symbol", tick.Symbol),
				zap.String("canonical", tick.Canonical),
			)
			// Quality check — log outlier, still write to CH for completeness
			qr := qc.Check(tick)
			if qr.Dropped {
				d.Log.Warn("tick dropped (bid > ask)",
					zap.String("broker", tick.Broker),
					zap.String("symbol", tick.Symbol),
				)
				return
			}
			_ = publisher.Publish(ctx, tick)
			chWriter.Write(tick)
			if d.RDB != nil {
				_ = d.RDB.Set(ctx, "quote:"+tick.Broker+":"+tick.Symbol, tick.GetBid().GetValue(), 60*time.Second)
			}
			// Feed bar aggregator — completed bars written to CH + NATS
			agg.AddTick(tick, func(bar Bar) {
				barTotal.WithLabelValues(bar.Broker, bar.Canonical, bar.Period).Inc()
				// Write bar to ClickHouse (use background ctx — handler ctx is short-lived)
				conn, err := chConn.Conn(context.Background())
				if err != nil {
					d.Log.Error("bar: chconn failed", zap.Error(err),
						zap.String("canonical", bar.Canonical), zap.String("period", bar.Period))
				} else {
					insCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					chBatch, err := conn.PrepareBatch(insCtx, "INSERT INTO md_bars")
					if err != nil {
						d.Log.Error("bar: prepare batch failed", zap.Error(err),
							zap.String("canonical", bar.Canonical), zap.String("period", bar.Period))
					} else {
						if err := chBatch.Append(
							bar.TenantID, bar.Broker, bar.SymbolRaw, bar.Canonical, bar.Period,
							uint64(bar.OpenTsUnixMs), uint64(bar.CloseTsUnixMs),
							bar.Open, bar.High, bar.Low, bar.Close,
							bar.Volume, bar.TickCount,
						); err != nil {
							d.Log.Error("bar: append failed", zap.Error(err),
								zap.String("canonical", bar.Canonical), zap.String("period", bar.Period))
						}
						if err := chBatch.Send(); err != nil {
							d.Log.Error("bar: send failed", zap.Error(err),
								zap.String("canonical", bar.Canonical), zap.String("period", bar.Period))
						}
					}
					cancel()
				}
				// Publish bar to NATS
				subject := "md.bar." + bar.Broker + "." + bar.Canonical + "." + bar.Period
				data := fmt.Sprintf(`{"broker":"%s","canonical":"%s","period":"%s","open":%.5f,"high":%.5f,"low":%.5f,"close":%.5f,"volume":%.2f}`,
					bar.Broker, bar.Canonical, bar.Period, bar.Open, bar.High, bar.Low, bar.Close, bar.Volume,
				)
				_ = publisher.PublishRaw(subject, []byte(data))
			})
		}

		// Load broker-specific symbol names from broker_symbols;
		// each broker uses its own naming (e.g. EURUSDm vs EURUSD).
		symbols := loadBrokerSymbols(d.PG.Pool, gw.Platform() == "mt4")
		if len(symbols) == 0 {
			symbols = []string{"EURUSD", "EURUSDm", "EURUSD."} // fallback
		}
		d.Log.Info("subscribing to symbols",
			zap.String("key", key),
			zap.Int("count", len(symbols)),
			zap.Strings("sample", firstN(symbols, 5)),
		)
		go func(key string, gw Gateway) {
			_ = gw.Subscribe(ctx, symbols, func(tick *pb.Tick) { handler(key, gw, tick) })
		}(key, gw)
	}

	// Heartbeat loop
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		backoff := time.Second
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for key, gw := range manager.Connections() {
					if err := gw.HealthCheck(ctx); err != nil {
						d.Log.Warn("heartbeat failed", zap.String("key", key), zap.Error(err))
						connStates[key] = false
						time.Sleep(backoff)
						if err := gw.Connect(ctx); err != nil {
							if backoff < 60*time.Second {
								backoff *= 2
							}
							continue
						}
						connStates[key] = true
						backoff = time.Second
					} else {
						connStates[key] = true
						backoff = time.Second
					}
				}
			}
		}
	}()

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		for _, v := range connStates {
			if !v {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte("not ready"))
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})

	return nil
}

// loadAccounts reads active accounts from PG and converts to AccountConfig.
func loadAccounts(pool *pgxpool.Pool) ([]AccountConfig, error) {
	rows, err := pool.Query(context.Background(), `
		SELECT a.broker_id::text, a.login, a.password, a.server, a.platform, a.tenant_id::text
		FROM accounts a
		WHERE a.status = 'connected' AND a.is_disabled = false
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []AccountConfig
	for rows.Next() {
		var ac AccountConfig
		if err := rows.Scan(&ac.Broker, &ac.Login, &ac.Password, &ac.Server, &ac.Platform, &ac.TenantID); err != nil {
			continue
		}
		ac.Host, ac.Port = splitHostPort(ac.Server, "443")
		accounts = append(accounts, ac)
	}
	return accounts, rows.Err()
}

// loadBrokerSymbols loads symbol_raw names from broker_symbols.
// For MT4 the mt4Gateway ignores the symbol list anyway (OnQuote is global),
// but we return the correct names for logging and future MT5 use.
func loadBrokerSymbols(pool *pgxpool.Pool, isMT4 bool) []string {
	if pool == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := pool.Query(ctx,
		`SELECT symbol_raw FROM broker_symbols
		 WHERE trade_mode = 3 AND partial = false
		 ORDER BY symbol_raw LIMIT 100`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var symbols []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err == nil {
			symbols = append(symbols, s)
		}
	}
	return symbols
}

func firstN(ss []string, n int) []string {
	if len(ss) <= n {
		return ss
	}
	return ss[:n]
}

func splitHostPort(hostPort, defaultPort string) (string, string) {
	for i := len(hostPort) - 1; i >= 0; i-- {
		if hostPort[i] == ':' {
			return hostPort[:i], hostPort[i+1:]
		}
	}
	return hostPort, defaultPort
}
