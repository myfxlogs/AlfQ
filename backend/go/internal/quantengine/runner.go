// Package quantengine wires factor-svc and strategy-svc into a single process.
package quantengine

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/alfq/backend/go/internal/common/bootstrap"
	"github.com/alfq/backend/go/internal/factorsvc"
	"github.com/alfq/backend/go/internal/strategysvc"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// RunQuantEngine wires factor + strategy services and registers /readyz on mux.
func RunQuantEngine(mux *http.ServeMux, d *bootstrap.Deps) error {
	ctx := context.Background()

	fCfg := factorsvc.Config{
		NatsURL: os.Getenv("NATS_URL"),
		Factors: []factorsvc.FactorDef{
			{Name: "sma20", Expression: "sma($close, 20)", Symbols: []string{"EURUSD"}},
			{Name: "sma60", Expression: "sma($close, 60)", Symbols: []string{"EURUSD"}},
			{Name: "rsi14", Expression: "rsi(14)", Symbols: []string{"EURUSD"}},
		},
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

	loader := strategysvc.NewLoader()
	allocator := strategysvc.NewAllocator()
	allocator.SetAccount("demo", 100000.0)
	allocator.AddStrategy("demo", "sma_cross", 0.3, 5.0, 0.1)

	d.Log.Info("quant-engine starting",
		zap.Int("runners", loader.Count()),
		zap.Int("factors", len(fCfg.Factors)),
	)

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})

	chWriter.Start(ctx)
	defer func() { _ = chWriter.Close() }() //nolint:errcheck

	go func() {
		if err := sub.Start(ctx); err != nil {
			d.Log.Error("subscriber error", zap.Error(err))
		}
	}()

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for _, id := range loader.List() {
					if runner := loader.Get(id); runner != nil {
						_, _ = runner.Evaluate(ctx, "EURUSD", 1.0)
					}
				}
			}
		}
	}()

	return nil
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

	// Try to add stream; ignore if already exists
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
