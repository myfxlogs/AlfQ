// Package mdgateway — gateway runner that wires connections and starts the heartbeat loop.
package mdgateway

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	mthubv1connect "github.com/alfq/backend/go/gen/alfq/mthub/v1/mthubv1connect"
	"github.com/alfq/backend/go/internal/common/bootstrap"
	"github.com/alfq/backend/go/internal/common/db/redis"
	"github.com/alfq/backend/go/internal/mdgateway/chmigrate"
	"github.com/alfq/backend/go/internal/mthub"
	"github.com/alfq/backend/go/internal/symbolsync"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

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
		// Set gateway role for cross-tenant account/symbol metadata access.
		_ = d.PG.SetRole(ctx, "gateway")

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
	var hubPG *pgxpool.Pool
	if d.PG != nil {
		hubPG = d.PG.Pool
	}
	hub := mthub.NewHub(lookupGW, hubPG, d.Log)
	events := mthub.NewOrderEventBroker()
	mtHubSvc := mthub.NewMtHubService(hub, events, d.Log)
	mux.Handle(mthubv1connect.NewMtHubServiceHandler(mtHubSvc))
	d.Log.Info("mthub registered on http mux")

	publisher := NewPublisher(d.Log, natsURL)
	if err := publisher.Connect(ctx); err != nil {
		d.Log.Warn("nats connect failed", zap.Error(err))
	} else {
		// Ensure JetStream streams for tick + bar persistence.
		publisher.EnsureStreams(d.Log)
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

	// Shared tick handler for both initial connect and hot-join.
	tickHandler := func(key string, gw Gateway, tick *pb.Tick) {
			tickTotal.WithLabelValues(tick.Broker, tick.Symbol).Inc()
			// Quality check — log outlier, still write to CH for completeness
			qr := qc.Check(tick)
			if qr.Dropped {
				d.Log.Warn("tick dropped (bid > ask)",
					zap.String("broker", tick.Broker),
					zap.String("symbol", tick.Symbol),
				)
				return
			}
			if err := publisher.Publish(ctx, tick); err != nil {
			d.Log.Warn("tick publish error", zap.Error(err), zap.String("symbol", tick.Symbol))
		}
			chWriter.Write(tick)
			if d.RDB != nil {
				_ = d.RDB.Set(ctx, "quote:"+tick.Broker+":"+tick.Symbol, tick.GetBid().GetValue(), 60*time.Second)
			}
			// RS03: feed live price to hub for position current-price display
			if tick.Canonical != "" {
				if bid, err := strconv.ParseFloat(tick.GetBid().GetValue(), 64); err == nil {
					ask, _ := strconv.ParseFloat(tick.GetAsk().GetValue(), 64)
					hub.UpdatePrice(tick.Canonical, bid, ask)
				}
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
							decimal.NewFromFloat(bar.Open),
							decimal.NewFromFloat(bar.High),
							decimal.NewFromFloat(bar.Low),
							decimal.NewFromFloat(bar.Close),
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
			// Publish bar to NATS JetStream as protobuf (consumed by quant-engine factorsvc.Subscriber)
			if err := publisher.PublishBar(bar); err != nil {
				d.Log.Warn("bar publish failed", zap.Error(err),
					zap.String("canonical", bar.Canonical), zap.String("period", bar.Period))
			}
			})
		}

	connStates := make(map[string]bool)
	for key, gw := range manager.Connections() {
		connStates[key] = false
		if err := gw.Connect(ctx); err != nil {
			d.Log.Error("connect failed", zap.String("key", key), zap.Error(err))
			continue
		}
		connStates[key] = true
		d.Log.Info("broker connected", zap.String("key", key))

		go func(gw Gateway) {
			syncCtx, syncCancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer syncCancel()
			svc := symbolsync.NewService(d.PG.Pool, d.Log)
			_ = svc.Sync(syncCtx, symbolsync.SyncParams{
				BrokerID: gw.BrokerID(), Platform: gw.Platform(),
				SessionID: gw.SessionID(), Conn: gw.Conn(),
			})
		}(gw)

		// Load broker-specific symbol names from broker_symbols;
		brokerID, _ := extractBrokerID(key)
		symbols := loadBrokerSymbols(d.PG.Pool, brokerID)
		if len(symbols) == 0 {
			d.Log.Error("no tradable symbols found for broker, skipping subscription",
				zap.String("broker_id", brokerID),
				zap.String("key", key),
			)
			continue // skip this broker instead of fallback
		}
		d.Log.Info("subscribing to symbols",
			zap.String("key", key),
			zap.Int("count", len(symbols)),
			zap.Strings("sample", firstN(symbols, 5)),
		)
		go func(key string, gw Gateway) {
			_ = gw.Subscribe(ctx, symbols, func(tick *pb.Tick) { tickHandler(key, gw, tick) })
		}(key, gw)
	}

	// Hot-join: event-driven via PG LISTEN/NOTIFY for new/removed accounts.
	// Uses trigger trg_account_change (011_account_notify_trigger.sql).
	// Falls back to 30s polling if PG is unavailable.
	go func() {
		knownKeys := make(map[string]bool)
		for k := range connStates {
			knownKeys[k] = true
		}

		connectAccount := func(a AccountConfig) {
			key := a.Broker + "-" + a.Login
			if knownKeys[key] {
				return
			}
			manager.AddGateway(a)
			gw := manager.Connections()[key]
			if gw == nil {
				return
			}
			if err := gw.Connect(ctx); err != nil {
				d.Log.Error("hotjoin: connect failed", zap.String("key", key), zap.Error(err))
				return
			}
			connStates[key] = true
			knownKeys[key] = true
			d.Log.Info("hotjoin: broker connected", zap.String("key", key))

			go func(gw Gateway) {
				syncCtx, syncCancel := context.WithTimeout(context.Background(), 120*time.Second)
				defer syncCancel()
				svc := symbolsync.NewService(d.PG.Pool, d.Log)
				_ = svc.Sync(syncCtx, symbolsync.SyncParams{
					BrokerID: gw.BrokerID(), Platform: gw.Platform(),
					SessionID: gw.SessionID(), Conn: gw.Conn(),
				})
			}(gw)

			symbols := loadBrokerSymbols(d.PG.Pool, gw.BrokerID())
			if len(symbols) > 0 {
				go func(key string, gw Gateway) {
					_ = gw.Subscribe(ctx, symbols, func(tick *pb.Tick) { tickHandler(key, gw, tick) })
				}(key, gw)
			}
		}

		// Try PG LISTEN; fall back to polling if unavailable.
		if d.PG != nil {
			conn, err := d.PG.Pool.Acquire(ctx)
			if err == nil {
				_, err = conn.Exec(ctx, "LISTEN account_changes")
				if err == nil {
					d.Log.Info("hotjoin: listening on PG account_changes")
					for {
						notif, waitErr := conn.Conn().WaitForNotification(ctx)
						if waitErr != nil {
							d.Log.Warn("hotjoin: listen lost, falling back to poll", zap.Error(waitErr))
							break
						}
						accountID := notif.Payload
						// Load the specific account that changed
						a, loadErr := loadAccountByID(d.PG.Pool, accountID)
						if loadErr != nil || a == nil {
							d.Log.Warn("hotjoin: load changed account failed",
								zap.String("account_id", accountID), zap.Error(loadErr))
							continue
						}
						if a.Status != "connected" || a.IsDisabled {
							continue
						}
						connectAccount(*a)
					}
					conn.Release()
				} else {
					conn.Release()
				}
			}
		}

		// Fallback polling (runs if LISTEN failed, or after listen loss)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if d.PG == nil {
					continue
				}
				accounts, err := loadAccounts(d.PG.Pool)
				if err != nil {
					d.Log.Warn("hotjoin: load accounts failed", zap.Error(err))
					continue
				}
				for _, a := range accounts {
					connectAccount(a)
				}
			}
		}
	}()

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
// loadAccountByID loads a single account by UUID.
func loadAccountByID(pool *pgxpool.Pool, id string) (*AccountConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	row := pool.QueryRow(ctx,
		`SELECT a.broker_id::text, a.login, a.password, a.server, a.platform, a.tenant_id::text, a.status, a.is_disabled
		 FROM accounts a WHERE a.id = $1`, id)
	var ac AccountConfig
	err := row.Scan(&ac.Broker, &ac.Login, &ac.Password, &ac.Server, &ac.Platform, &ac.TenantID, &ac.Status, &ac.IsDisabled)
	if err != nil {
		return nil, err
	}
	ac.Host, ac.Port = splitHostPort(ac.Server, "443")
	return &ac, nil
}

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

// loadBrokerSymbols loads symbol_raw names from broker_symbols per broker.
// For MT4 the mt4Gateway ignores the symbol list anyway (OnQuote is global),
// but we return the correct names for logging and future MT5 use.
func loadBrokerSymbols(pool *pgxpool.Pool, brokerID string) []string {
	if pool == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := pool.Query(ctx,
		`SELECT symbol_raw FROM broker_symbols
		 WHERE broker_id = $1 AND partial = false AND digits > 0
		 ORDER BY symbol_raw LIMIT 200`,
		brokerID,
	)
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

// extractBrokerID parses "brokerID-login" or returns the full key.
func extractBrokerID(key string) (string, string) {
	for i := len(key) - 1; i >= 0; i-- {
		if key[i] == '-' {
			return key[:i], key[i+1:]
		}
	}
	return key, key
}
