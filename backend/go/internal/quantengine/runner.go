// Package quantengine wires factor-svc and strategy-svc into a single process.
// EP-2: Integrates strategy spec loading, signal generation, and OMS wiring.
package quantengine

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/alfq/backend/go/internal/common/bootstrap"
	"github.com/alfq/backend/go/internal/factorsvc"
	stratspec "github.com/alfq/backend/go/internal/strategysvc/spec"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// StrategyRuntime bundles a loaded spec with its model runner.
type StrategyRuntime struct {
	Spec   *stratspec.StrategySpec
	Runner *ModelRunner
	// mu     sync.Mutex // reserved for future concurrent access
}

// SignalHandler receives signals and routes them to OMS.
type SignalHandler func(symbol string, side string, qty float64, reason string)

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

	// Build runtime map
	runtimes := make(map[string]*StrategyRuntime, len(specs))
	factorDefs := make([]factorsvc.FactorDef, 0)

	for _, spec := range specs {
		mr, err := NewModelRunner(spec)
		if err != nil {
			d.Log.Warn("model runner creation failed", zap.String("spec", spec.Name), zap.Error(err))
			continue
		}
		runtimes[spec.Name] = &StrategyRuntime{Spec: spec, Runner: mr}

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

	// Ensure NATS JetStream stream exists for md.bar.>
	ensureBarStream(fCfg.NatsURL, d.Log)

	sub := factorsvc.NewSubscriber(engine, fCfg.NatsURL, d.Log)

	chWCfg := factorsvc.DefaultFactorCHWriterConfig()
	chWriter := factorsvc.NewFactorCHWriter(chWCfg, d.Log)

	d.Log.Info("quant-engine starting",
		zap.Int("runtimes", len(runtimes)),
		zap.Int("factors", len(fCfg.Factors)),
	)

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})

	chWriter.Start(ctx)
	defer func() { _ = chWriter.Close() }()

	go func() {
		if err := sub.Start(ctx); err != nil {
			d.Log.Error("subscriber error", zap.Error(err))
		}
	}()

	// ── Main evaluation loop: evaluate factors → generate signals ──
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				evaluateAll(ctx, engine, runtimes, onSignal, d.Log)
			}
		}
	}()

	return nil
}

func evaluateAll(ctx context.Context, engine *factorsvc.Engine, runtimes map[string]*StrategyRuntime, onSignal SignalHandler, log *zap.Logger) {
	for name, rt := range runtimes {
		// Evaluate all factors for this strategy's symbols
		factorVals := make(map[string]float64)
		for _, sym := range rt.Spec.CanonicalSymbols {
			// Factor evaluation uses the engine's bar stream.
			// For demo: use a single symbol's latest bar.
			_ = sym
		}

		// In production this would come from NATS bar stream.
		// For now, run DSL signal evaluation.
		signal, err := rt.Runner.Predict(ctx, factorVals)
		if err != nil {
			log.Warn("signal eval failed", zap.String("strategy", name), zap.Error(err))
			continue
		}

		dir := Direction(signal)
		if dir == "flat" {
			continue
		}

		qty := 0.1 // default 0.1 lots; in prod use sizing calculator
		if onSignal != nil {
			// Route to OMS via signal handler
			onSignal(rt.Spec.CanonicalSymbols[0], dir, qty, name)
		}

		log.Debug("signal generated",
			zap.String("strategy", name),
			zap.Float64("signal", signal),
			zap.String("direction", dir),
		)
	}
}

func defaultDemoSpec() *stratspec.StrategySpec {
	return &stratspec.StrategySpec{
		Name:             "demo_sma_cross",
		Version:          "1.0.0",
		CanonicalSymbols: []string{"EURUSD"},
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
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Warn("nats connect for stream setup failed", zap.Error(err))
		return
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		log.Warn("nats jetstream for stream setup failed", zap.Error(err))
		return
	}

	_, err = js.AddStream(&nats.StreamConfig{
		Name:     "MD_BARS",
		Subjects: []string{"md.bar.>"},
		MaxMsgs:  1_000_000,
		MaxBytes: 512 * 1024 * 1024,
		Storage:  nats.FileStorage,
	})
	if err != nil {
		log.Warn("nats add stream MD_BARS", zap.Error(err))
	} else {
		log.Info("nats stream MD_BARS created")
	}
}
