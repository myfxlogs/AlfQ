// md-gateway is the ALFQ market data gateway.
// It connects to broker mtapi endpoints and publishes normalized ticks to NATS.
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/alfq/backend/go/internal/common/db/redis"
	"github.com/alfq/backend/go/internal/common/logger"
	"github.com/alfq/backend/go/internal/mdgateway"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

func main() {
	// Load config from YAML (configs/md-gateway.yaml)
	v := viper.New()
	v.SetConfigName("md-gateway")
	v.SetConfigType("yaml")
	v.AddConfigPath("configs")
	v.SetDefault("server.listen", ":9001")
	v.SetDefault("log.level", "info")
	v.SetDefault("nats.url", "nats://localhost:4222")
	if err := v.ReadInConfig(); err != nil {
		zap.L().Warn("config not found, using defaults", zap.Error(err))
	}

	log, err := logger.New(v.GetString("log.level"))
	if err != nil {
		zap.L().Error("logger init failed", zap.Error(err))
		os.Exit(1)
	}
	defer log.Sync()

	// Build account entries from config
	var accounts []mdgateway.AccountEntry
	if err := v.UnmarshalKey("accounts", &accounts); err != nil {
		log.Warn("failed to unmarshal accounts, using empty list", zap.Error(err))
	}
	mgrCfg := mdgateway.Config{
		Log:      mdgateway.LogConfig{Level: v.GetString("log.level")},
		Accounts: accounts,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Redis connection for latest quote snapshots.
	redisAddr := v.GetString("redis.addr")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redisClient, err := redis.Connect(ctx, redisAddr, "")
	if err != nil {
		log.Warn("redis connect failed, snapshots disabled", zap.Error(err))
	}
	if redisClient != nil {
		defer redisClient.Close()
	}

	manager := mdgateway.NewManager(mgrCfg)
	publisher := mdgateway.NewPublisher(log, v.GetString("nats.url"))
	if err := publisher.Connect(ctx); err != nil {
		log.Warn("nats connect failed, ticks will be logged only", zap.Error(err))
	}

	chCfg := mdgateway.DefaultCHWriterConfig()
	chWriter, err := mdgateway.NewCHWriter(chCfg, log)
	if err != nil {
		log.Error("chwriter init failed", zap.Error(err))
		os.Exit(1)
	}
	chWriter.Start(ctx)
	defer chWriter.Close()

	// Prometheus metrics.
	tickTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "md_tick_total", Help: "Total normalized ticks received",
	}, []string{"broker", "symbol"})
	prometheus.MustRegister(tickTotal)

	// Track connection states for /readyz.
	connStates := make(map[string]bool)

	for key, gw := range manager.Connections() {
		connStates[key] = false
		if err := gw.Connect(ctx); err != nil {
			log.Error("connect failed", zap.String("key", key), zap.Error(err))
			continue
		}
		connStates[key] = true
		log.Info("broker connected", zap.String("key", key), zap.String("platform", gw.Platform()))

		handler := func(key string, gw mdgateway.Gateway, tick *pb.Tick) {
			tickTotal.WithLabelValues(tick.Broker, tick.Symbol).Inc()
			if err := publisher.Publish(ctx, tick); err != nil {
				log.Warn("publish failed", zap.Error(err))
			}
			chWriter.Write(tick)
			if redisClient != nil {
				redisClient.Set(ctx, "quote:"+tick.Broker+":"+tick.Symbol, tick.GetBid().GetValue(), 60*time.Second)
			}
		}

		// Use symbols from account config; fallback to EURUSD.
		symbols := []string{"EURUSD"}
		for _, acc := range accounts {
			if acc.Broker+"-"+acc.Login == key && len(acc.Symbols) > 0 {
				symbols = acc.Symbols
				break
			}
		}

		go func(key string, gw mdgateway.Gateway) {
			if err := gw.Subscribe(ctx, symbols, func(tick *pb.Tick) {
				handler(key, gw, tick)
			}); err != nil {
				log.Error("subscribe failed", zap.Error(err))
			}
		}(key, gw)
	}

	// Heartbeat loop with exponential backoff reconnect.
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
						log.Warn("heartbeat failed", zap.String("key", key), zap.Error(err))
						connStates[key] = false
						// Attempt reconnect with backoff.
						time.Sleep(backoff)
						if err := gw.Connect(ctx); err != nil {
							log.Error("reconnect failed", zap.String("key", key), zap.Error(err))
							if backoff < 60*time.Second {
								backoff *= 2
							}
							continue
						}
						connStates[key] = true
						backoff = time.Second
						log.Info("reconnected", zap.String("key", key))
					} else {
						connStates[key] = true
						backoff = time.Second
					}
				}
			}
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ready := true
		for _, v := range connStates {
			if !v {
				ready = false
				break
			}
		}
		if !ready {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("not ready"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{Addr: v.GetString("server.listen"), Handler: mux}
	go func() {
		log.Info("md-gateway starting", zap.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	log.Info("shutting down...")

	shutdownCtx, sdCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer sdCancel()

	server.Shutdown(shutdownCtx)
	publisher.Close()
	for _, gw := range manager.Connections() {
		gw.Disconnect(shutdownCtx)
	}

	log.Info("md-gateway stopped")
}
