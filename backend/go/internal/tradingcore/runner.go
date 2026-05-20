// Package tradingcore wires trading-core dependencies (adminapi, oms, risksvc, accountconn)
// into a single process behind a connectRPC h2c mux.
package tradingcore

import (
	"context"
	"net/http"
	"os"

	"github.com/alfq/backend/go/gen/alfq/v1/alfqv1connect"
	"github.com/alfq/backend/go/internal/accountconn"
	"github.com/alfq/backend/go/internal/adminapi"
	"github.com/alfq/backend/go/internal/common/bootstrap"
	"github.com/alfq/backend/go/internal/common/bus"
	"github.com/alfq/backend/go/internal/common/config"
	"github.com/alfq/backend/go/internal/ssehub"
	"github.com/alfq/backend/go/internal/common/health"
	"github.com/alfq/backend/go/internal/oms"
	"github.com/alfq/backend/go/internal/oms/repo"
	"github.com/alfq/backend/go/internal/risksvc"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// RunTradingCore wires all trading-core dependencies and registers routes on mux.
// Returns a shutdown function that the caller should defer.
func RunTradingCore(mux *http.ServeMux, d *bootstrap.Deps) (shutdown func(), err error) {
	// Health (ConnectRPC)
	path, handler := alfqv1connect.NewHealthServiceHandler(&health.Service{})
	mux.Handle(path, handler)

	// Config
	cfg := config.Defaults()
	if cfgPath := os.Getenv("ALFQ_CONFIG"); cfgPath != "" {
		config.Load(cfgPath, cfg)
	}

	// NATS
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}
	nc, err := nats.Connect(natsURL)
	if err != nil {
		d.Log.Warn("nats connect failed", zap.Error(err))
	}
	var js nats.JetStreamContext
	if nc != nil {
		js, err = nc.JetStream()
		if err != nil {
			d.Log.Warn("nats jetstream failed", zap.Error(err))
		}
	}

	// NATS bus (for legacy compatibility)
	busClient, err := bus.Connect(context.Background(), natsURL)
	if err != nil {
		d.Log.Warn("nats bus connect failed", zap.Error(err))
	}

	// OMS + Risk
	if oms.IsTerminal(0) {
		d.Log.Info("oms state machine loaded")
	}
	_ = repo.NewOrderRepo(d.PG)
	_ = repo.NewPositionRepo(d.PG)

	engine := risksvc.NewEngine()
	kill := &risksvc.KillSwitch{}
	breaker := risksvc.NewBreaker(10)
	_ = breaker
	_ = risksvc.NewKillExecutor()
	_ = risksvc.NewEventRecorder()

	d.Log.Info("risk engine loaded",
		zap.Int("rules", 10),
		zap.Bool("kill_active", kill.IsActive()),
		zap.Bool("breaker_ok", breaker.Allow()),
	)

	// Account connection manager
	acctMgr := accountconn.NewManager(d.Log, d.PG, d.RDB, nc, js, cfg.MT4Gateway, cfg.MT5Gateway)

	// Reconnect all currently connected accounts on startup
	go func() {
		rows, err := d.PG.Query(context.Background(), `
			SELECT a.id, a.login, a.password, a.server, a.platform
			FROM accounts a
			WHERE a.status IN ('connected', 'error') AND a.is_disabled = false
		`)
		if err != nil {
			d.Log.Warn("startup reconnect query failed", zap.Error(err))
			return
		}
		defer rows.Close()
		for rows.Next() {
			var info accountconn.AccountInfo
			if err := rows.Scan(&info.ID, &info.Login, &info.Password, &info.Server, &info.Platform); err != nil {
				d.Log.Warn("startup reconnect scan failed", zap.Error(err))
				continue
			}
			acctMgr.Connect(context.Background(), info)
		}
		d.Log.Info("startup reconnect complete", zap.Int("count", acctMgr.ActiveCount()))
	}()

	// Admin API handlers
	svc := adminapi.NewService(d.PG).WithGateways(cfg.MT4Gateway, cfg.MT5Gateway)
	svc.WithLog(d.Log)
	svc.WithAccountConnector(&acctAdapter{acctMgr}) // wire account connector

	adp := adminapi.NewAdapter(svc)
	mux.Handle(alfqv1connect.NewBrokerServiceHandler(adp))
	mux.Handle(alfqv1connect.NewAccountServiceHandler(adp))
	mux.Handle(alfqv1connect.NewStrategyServiceHandler(adp))
	mux.Handle(alfqv1connect.NewBacktestServiceHandler(adp))
	mux.Handle(alfqv1connect.NewAuditServiceHandler(adp))
	mux.Handle(alfqv1connect.NewSystemSettingsServiceHandler(adp))
	mux.Handle(alfqv1connect.NewServiceManagementServiceHandler(adp))

	// SSE hub for real-time account status push
	sse := ssehub.New()
	mux.HandleFunc("/sse/accounts", sse.ServeHTTP)

	// Subscribe NATS to fan account.status.* → SSE
	if nc != nil {
		_, err := nc.Subscribe("account.status.*", func(msg *nats.Msg) {
			sse.Broadcast(msg.Data)
		})
		if err != nil {
			d.Log.Warn("sse nats subscribe failed", zap.Error(err))
		} else {
			d.Log.Info("sse hub started", zap.String("path", "/sse/accounts"))
		}
	}

	// Auth
	authH, err := adminapi.NewAuthHandler(d.PG, d.RDB)
	if err != nil {
		d.Log.Warn("auth handler unavailable", zap.Error(err))
	} else {
		authPath, authHandler := alfqv1connect.NewAuthServiceHandler(authH)
		mux.Handle(authPath, authHandler)
		d.Log.Info("auth service registered", zap.String("path", authPath))
		d.Middleware = authH.AuthMiddleware
	}

	// /readyz with kill-switch awareness
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if kill.IsActive() {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("kill switch active"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})

	_ = engine

	shutdown = func() {
		acctMgr.Shutdown()
		if nc != nil {
			nc.Close()
		}
		if busClient != nil {
			busClient.Close()
		}
	}
	return shutdown, nil
}

// acctAdapter adapts accountconn.Manager to adminapi.AccountConnector.
type acctAdapter struct {
	mgr *accountconn.Manager
}

func (a *acctAdapter) Connect(ctx context.Context, info adminapi.AccountInfo) {
	a.mgr.Connect(ctx, accountconn.AccountInfo{
		ID: info.ID, Login: info.Login, Password: info.Password,
		Server: info.Server, Platform: info.Platform,
	})
}

func (a *acctAdapter) Disconnect(accountID string) {
	a.mgr.Disconnect(accountID)
}
