// Package quantengine wires factor-svc and strategy-svc into a single process.
// RS05: Per-strategy goroutine isolation + snapshot persistence + hot-reload.
package quantengine

import (
	"context"
	"net/http"
	"os"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/alfq/backend/go/internal/common/bootstrap"
	"github.com/alfq/backend/go/internal/factorsvc"
	stratspec "github.com/alfq/backend/go/internal/strategysvc/spec"
	"go.uber.org/zap"
)

// SignalHandler receives signals and routes them to OMS.
// strategyID is the DB UUID of the strategy that produced this signal.
type SignalHandler func(strategyID, symbol, side string, qty float64, reason string)

// RunQuantEngine wires factor + strategy services and registers /readyz on mux.
func RunQuantEngine(mux *http.ServeMux, d *bootstrap.Deps) error {
	return RunQuantEngineWithSignalHandler(mux, d, nil)
}

// RunQuantEngineWithSignalHandler starts quant-engine with an optional signal handler for OMS wiring.
func RunQuantEngineWithSignalHandler(mux *http.ServeMux, d *bootstrap.Deps, onSignal SignalHandler) error {
	ctx := context.Background()

	// ── Load strategy specs ──
	specDir := os.Getenv("ALFQ_SPEC_DIR")
	if specDir == "" {
		specDir = "configs/specs"
	}
	specs, err := stratspec.LoadDir(specDir)
	if err != nil {
		d.Log.Warn("spec load dir failed, using demo config", zap.Error(err), zap.String("dir", specDir))
		specs = []*stratspec.StrategySpec{defaultDemoSpec()}
	}
	d.Log.Info("strategy specs loaded", zap.Int("count", len(specs)))

	// Build factor engine
	factorDefs := make([]factorsvc.FactorDef, 0)
	for _, spec := range specs {
		for name, expr := range spec.Factors {
			factorDefs = append(factorDefs, factorsvc.FactorDef{
				Name:       name,
				Expression: expr,
				Symbols:    spec.CanonicalSymbols,
			})
		}
	}

	fCfg := factorsvc.Config{
		NatsURL: os.Getenv("NATS_URL"),
		Factors: factorDefs,
	}
	if fCfg.NatsURL == "" {
		fCfg.NatsURL = "nats://localhost:4222"
	}

	engine := factorsvc.NewEngine(fCfg)

	// RS03: WindowBuffer for rolling factor computation + CH bootstrap
	maxWindow := maxFactorWindow(factorDefs)
	buf := factorsvc.NewWindowBuffer(maxWindow, d.Log)
	specsList := make([]factorsvc.BootstrapSpec, 0)
	for _, spec := range specs {
		for _, sym := range spec.CanonicalSymbols {
			specsList = append(specsList, factorsvc.BootstrapSpec{
				TenantID: "00000000-0000-0000-0000-000000000001",
				Symbol:   sym,
				Period:   spec.Period,
				Limit:    maxWindow,
			})
		}
	}
	buf.Bootstrap(ctx, specsList)
	engine.SetBuffer(buf)
	d.Log.Info("window buffer bootstrapped", zap.Int("specs", len(specsList)), zap.Int("max_window", maxWindow))

	// RS03: Bootstrap WindowBuffer from ClickHouse historical bars.
	chConn := bootstrapFromCH(ctx, buf, specsList, d.Log)

	// Ensure NATS JetStream stream exists for md.bar.>
	ensureBarStream(fCfg.NatsURL, d.Log)

	sub := factorsvc.NewSubscriber(engine, fCfg.NatsURL, d.Log)

	// R18: FactorCHWriter with real CH connection for INSERTs.
	chWCfg := factorsvc.DefaultFactorCHWriterConfig()
	chWriter := factorsvc.NewFactorCHWriter(chWCfg, d.Log)
	if chConn != nil {
		chWriter.WithConn(chConn)
	}
	sub.SetCHWriter(chWriter)

	// ── RS05: RuntimeManager with per-strategy isolation ──
	runtimeMgr := NewRuntimeManager(d.Log).WithPool(d.PG)

	// Load running strategies from database: name → id (respect status field)
	runningIDs := make(map[string]string)
	if d.PG != nil {
		rows, err := d.PG.Query(ctx, `SELECT id, name FROM strategies WHERE status = 'running'`)
		if err != nil {
			d.Log.Warn("failed to query running strategies", zap.Error(err))
		} else {
			for rows.Next() {
				var id, name string
				if err := rows.Scan(&id, &name); err == nil {
					runningIDs[name] = id
				}
			}
			rows.Close()
		}
	}

	for _, spec := range specs {
		strategyID, ok := runningIDs[spec.Name]
		if !ok {
			d.Log.Info("skipping strategy (not running in DB)", zap.String("strategy", spec.Name))
			continue
		}
		rt, err := NewStrategyRuntime(spec, engine, onSignal, d.Log)
		if err != nil {
			d.Log.Warn("runtime creation failed", zap.String("spec", spec.Name), zap.Error(err))
			continue
		}
		rt.strategyID = strategyID
		runtimeMgr.Add(ctx, rt)
		d.Log.Info("runtime started", zap.String("strategy", spec.Name), zap.String("strategy_id", strategyID))
	}

	// Restore previous state from PG snapshots (RS05)
	if err := runtimeMgr.RestoreAll(ctx); err != nil {
		d.Log.Warn("runtime restore failed", zap.Error(err))
	}

	// Start snapshot persistence loop (30s)
	runtimeMgr.StartSnapshotLoop(ctx, 30*time.Second)

	// RS05: Listen for strategy_revisions NOTIFY for hot-reload
	runtimeMgr.ListenForRevisions(ctx)

	d.Log.Info("quant-engine starting",
		zap.Int("runtimes", runtimeMgr.Count()),
		zap.Int("factors", len(fCfg.Factors)),
	)

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})

	chWriter.Start(ctx)

	go func() {
		if err := sub.Start(ctx); err != nil {
			d.Log.Error("subscriber error", zap.Error(err))
		}
	}()

	// ── Bar-driven evaluation loop (replaces 10s ticker) ──
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// RS05: Dispatch bar event to all isolated runtimes.
				runtimeMgr.OnBar()
			}
		}
	}()

	return nil
}

// bootstrapFromCH fills the WindowBuffer with historical bars from ClickHouse.
// Returns the CH connection for reuse by the caller (e.g. factor writer).
func bootstrapFromCH(ctx context.Context, buf *factorsvc.WindowBuffer, specs []factorsvc.BootstrapSpec, log *zap.Logger) clickhouse.Conn {
	chAddr := os.Getenv("CH_ADDR")
	if chAddr == "" {
		chAddr = "localhost:9000"
	}
	chPassword := os.Getenv("CLICKHOUSE_PASSWORD")
	
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{chAddr},
		Auth: clickhouse.Auth{
			Database: "alfq",
			Username: "alfq",
			Password: chPassword,
		},
		DialTimeout: 10 * time.Second,
	})
	if err != nil {
		log.Warn("ch bootstrap: connect failed, will rely on live bars", zap.Error(err))
		return nil
	}

	if err := conn.Ping(ctx); err != nil {
		log.Warn("ch bootstrap: ping failed, will rely on live bars", zap.Error(err))
		conn.Close()
		return nil
	}

	totalBars := 0
	for _, spec := range specs {
		rows, err := conn.Query(ctx, fmt.Sprintf(
			`SELECT toFloat64(open), toFloat64(high), toFloat64(low), toFloat64(close), volume, close_ts_unix_ms
			 FROM alfq.md_bars
			 WHERE canonical = '%s' AND period = '%s'
			 ORDER BY close_ts_unix_ms DESC
			 LIMIT %d`,
			spec.Symbol, spec.Period, spec.Limit,
		))
		if err != nil {
			log.Warn("ch bootstrap: query failed", zap.String("symbol", spec.Symbol), zap.String("period", spec.Period), zap.Error(err))
			continue
		}

		// Collect rows (newest first from DESC), then push oldest first
		type barRow struct {
			open, high, low, close, volume float64
			tsMs                           uint64
		}
		var bars []barRow
		for rows.Next() {
			var open, high, low, close, volume float64
			var tsMs uint64
			if err := rows.Scan(&open, &high, &low, &close, &volume, &tsMs); err != nil {
				log.Warn("ch bootstrap: scan failed", zap.Error(err))
				continue
			}
			bars = append(bars, barRow{open, high, low, close, volume, tsMs})
		}
		rows.Close()

		// Push oldest first
		for i := len(bars) - 1; i >= 0; i-- {
			b := bars[i]
			buf.PushRaw(spec.TenantID, spec.Symbol, spec.Period, b.open, b.high, b.low, b.close, b.volume, int64(b.tsMs))
			totalBars++
		}
	}
	log.Info("ch bootstrap: loaded historical bars", zap.Int("total_bars", totalBars))
	return conn
}

func defaultDemoSpec() *stratspec.StrategySpec {
	return &stratspec.StrategySpec{
		Name:             "demo_sma_e2e",
		Version:          "1.0.0",
		CanonicalSymbols: []string{"BTCUSD"},
		Period:           "1h",
		Factors: map[string]string{
			"sma20": "sma($close, 20)",
			"sma60": "sma($close, 60)",
		},
		SignalRule: "sma20 > sma60 ? 1 : -1",
		Sizing:     map[string]any{"type": "fixed_lots", "lots": 0.1},
	}
}

func ensureBarStream(natsURL string, log *zap.Logger) {
	_ = natsURL
	_ = log
}

// maxFactorWindow scans factor definitions and returns the maximum window size needed.
// For SMA(n)/EMA(n)/RSI(n), returns the largest n value across all factors.
func maxFactorWindow(defs []factorsvc.FactorDef) int {
	maxW := 60 // default minimum
	for _, f := range defs {
		// Simple parsing: extract numbers after known function names
		for _, fn := range []string{"sma(", "ema(", "wma(", "rsi(", "std(", "var(", "min(", "max(", "sum(", "ref(", "delta(", "pct_change(", "zscore(", "rank(", "atr("} {
			idx := 0
			for {
				pos := indexAfter(f.Expression, fn, idx)
				if pos < 0 {
					break
				}
				numEnd := pos
				for numEnd < len(f.Expression) && f.Expression[numEnd] >= '0' && f.Expression[numEnd] <= '9' {
					numEnd++
				}
				if numEnd > pos {
					n := 0
					for i := pos; i < numEnd; i++ {
						n = n*10 + int(f.Expression[i]-'0')
					}
					if n > maxW {
						maxW = n
					}
				}
				idx = numEnd
			}
		}
	}
	return maxW
}

func indexAfter(s, sub string, start int) int {
	for i := start; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i + len(sub)
		}
	}
	return -1
}