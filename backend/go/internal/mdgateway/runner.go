// Package mdgateway — gateway runner that wires connections and starts the heartbeat loop.
package mdgateway

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/alfq/backend/go/internal/common/bootstrap"
	"github.com/alfq/backend/go/internal/common/db/redis"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

// RunGateway wires all gateway connections, starts the heartbeat, and registers /readyz.
func RunGateway(mux *http.ServeMux, d *bootstrap.Deps, cfg Config, natsURL, redisAddr string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if d.RDB == nil {
		redisClient, err := redis.Connect(ctx, redisAddr, "")
		if err == nil {
			d.RDB = redisClient
			defer func() { _ = redisClient.Close() }() //nolint:errcheck
		}
	}

	manager := NewManager(cfg)
	publisher := NewPublisher(d.Log, natsURL)
	if err := publisher.Connect(ctx); err != nil {
		d.Log.Warn("nats connect failed", zap.Error(err))
	}

	chCfg := DefaultCHWriterConfig()
	chWriter, err := NewCHWriter(chCfg, d.Log)
	if err != nil {
		return fmt.Errorf("chwriter: %w", err)
	}
	chWriter.Start(ctx)
	defer func() { _ = chWriter.Close() }() //nolint:errcheck

	tickTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "md_tick_total", Help: "Total normalized ticks received",
	}, []string{"broker", "symbol"})
	prometheus.MustRegister(tickTotal)

	connStates := make(map[string]bool)
	accounts := cfg.Accounts
	for key, gw := range manager.Connections() {
		connStates[key] = false
		if err := gw.Connect(ctx); err != nil {
			d.Log.Error("connect failed", zap.String("key", key), zap.Error(err))
			continue
		}
		connStates[key] = true
		d.Log.Info("broker connected", zap.String("key", key))

		handler := func(key string, gw Gateway, tick *pb.Tick) {
			tickTotal.WithLabelValues(tick.Broker, tick.Symbol).Inc()
			_ = publisher.Publish(ctx, tick)
			chWriter.Write(tick)
			if d.RDB != nil {
				d.RDB.(*redis.Client).Set(ctx, "quote:"+tick.Broker+":"+tick.Symbol, tick.GetBid().GetValue(), 60*time.Second)
			}
		}

		symbols := []string{"EURUSD"}
		for _, acc := range accounts {
			if acc.Broker+"-"+acc.Login == key && len(acc.Symbols) > 0 {
				symbols = acc.Symbols
				break
			}
		}
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
