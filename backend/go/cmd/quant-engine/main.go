// quant-engine merges factor-svc and strategy-svc into a single process.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/alfq/backend/go/internal/common/bootstrap"
	"github.com/alfq/backend/go/internal/factorsvc"
	"github.com/alfq/backend/go/internal/strategysvc"
	"go.uber.org/zap"
)

func main() {
	if err := bootstrap.Run("quant-engine", register,
		bootstrap.WithoutPG(),
		bootstrap.WithoutRedis(),
	); err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap: %v\n", err)
		os.Exit(1)
	}
}

func register(adapter *bootstrap.ServeMuxAdapter, d *bootstrap.Deps) error {
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

	chWriter.Start(ctx)
	defer chWriter.Close()

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
