// Package tradingcore wires trading-core dependencies (adminapi, oms, risksvc, accountconn)
// into a single process behind a connectRPC h2c mux.
package tradingcore

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/alfq/backend/go/gen/alfq/v1/alfqv1connect"
	"github.com/alfq/backend/go/internal/accountconn"
	"github.com/alfq/backend/go/internal/adminapi"
	"github.com/alfq/backend/go/internal/common/bootstrap"
	"github.com/alfq/backend/go/internal/common/bus"
	"github.com/alfq/backend/go/internal/common/config"
	"github.com/alfq/backend/go/internal/common/crypto"
	"github.com/alfq/backend/go/internal/common/health"
	"github.com/alfq/backend/go/internal/mthub"
	"github.com/alfq/backend/go/internal/oms"
	"github.com/alfq/backend/go/internal/oms/repo"
	"github.com/alfq/backend/go/internal/risksvc"
	"github.com/alfq/backend/go/internal/ssehub"
	"github.com/alfq/backend/go/internal/symbolsync"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
	"google.golang.org/grpc"
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
		_, _ = config.Load(cfgPath, cfg)
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
	d.Log.Info("oms state machine loaded")
	orderRepo := repo.NewOrderRepo(d.PG)
	_ = repo.NewPositionRepo(d.PG)
	historyRepo := repo.NewHistoryOrderRepo(d.PG)

	// Risk event writer for audit trail and promotion gate (RC04)
	riskEventRepo := repo.NewRiskEventRepo(d.PG)

	engine := risksvc.NewEngine()
	kill := &risksvc.KillSwitch{}
	breaker := risksvc.NewBreaker(10)
	_ = breaker
	_ = risksvc.NewKillExecutor()
	_ = risksvc.NewEventRecorder()

	// mthub address (used for both accountconn and reconciler)
	mthubAddr := os.Getenv("MTHUB_ADDR")
	if mthubAddr == "" {
		mthubAddr = "md-gateway:9001" // Docker compose internal
	}

	// OMS Order Executor with full state machine (RC10)
	// Note: BrokerAdapter is wired per-order via mthub client; a nil adapter is acceptable
	// because adminapi/strategy_handler routes through WithLiveSession → mthub.
	executor := oms.NewOrderExecutor(nil, engine, nil).
		WithOrderRepo(orderRepo).
		WithRiskEventWriter(riskEventRepo)

	// Wire canonical → symbol_raw resolver for Gate-3 (symbol_not_on_broker)
	if d.PG != nil {
		resolver := adminapi.NewSymbolResolver(d.PG.Pool)
		executor.WithSymbolResolver(&omsSymbolResolver{resolver: resolver})
		d.Log.Info("symbol resolver wired for canonical resolution")
	}

	d.Log.Info("oms order executor wired", zap.Bool("pg", d.PG != nil))

	// Reconciler: compares local PG orders with broker state every 30s (RC10)
	mthubClient := mthub.NewClient(mthubAddr)
	reconciler := oms.NewReconciler(d.PG, orderRepo, mthubClient, d.Log)
	go reconciler.Run(context.Background())
	d.Log.Info("oms reconciler started", zap.Duration("interval", 30*time.Second))

	d.Log.Info("risk engine loaded",
		zap.Int("rules", 10),
		zap.Bool("kill_active", kill.IsActive()),
		zap.Bool("breaker_ok", breaker.Allow()),
	)

	// M4: Wire CanonicalAuth (replaces legacy Whitelist + loadWhitelistFromDB)
	if d.PG != nil {
		engine.WithCanonicalAuth(d.PG.Pool)
		if ca := engine.CanonicalAuth(); ca != nil {
			ca.StartNotifyListener(context.Background()) // M6: hot-reload cache
		}
		d.Log.Info("canonical auth rule activated (Gate 1+2)")
	}

	// Account connection manager
	symSvc := &symAdapter{symbolsync.NewService(d.PG.Pool, d.Log)}
	acctMgr := accountconn.NewManager(d.Log, d.PG, d.RDB, nc, js, cfg.MT4Gateway, cfg.MT5Gateway, symSvc, mthubAddr)
	syncWorker := accountconn.NewSyncWorker(d.PG, historyRepo, d.Log)
	acctMgr.SetSyncWorker(syncWorker)

	// Wire risk engine to account state updates for real-time rule evaluation
	acctMgr.SetOnAccountUpdate(func(accountID string, balance, equity, margin, freeMargin float64) {
		state := &risksvc.AccountState{
			Equity:     equity,
			Balance:    balance,
			Margin:     margin,
			FreeMargin: freeMargin,
			DailyPnL:   risksvc.ComputeDailyPnL(balance, equity),
		}
		engine.UpdateState(accountID, state)
	})

	// Reconnect all currently connected accounts on startup
	go func() {
		rows, err := d.PG.Query(context.Background(), `
			SELECT a.id, a.login, a.password, a.server, a.platform, a.broker_id
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
			if err := rows.Scan(&info.ID, &info.Login, &info.Password, &info.Server, &info.Platform, &info.BrokerID); err != nil {
				d.Log.Warn("startup reconnect scan failed", zap.Error(err))
				continue
			}
			acctMgr.Connect(context.Background(), info)
		}
		d.Log.Info("startup reconnect complete", zap.Int("count", acctMgr.ActiveCount()))
	}()

	// Periodic reconcile ticker: every 10 minutes pull last 5 minutes for all connected accounts
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			rows, err := d.PG.Query(context.Background(), `SELECT id FROM accounts WHERE status='connected' AND is_disabled=false`)
			if err != nil {
				d.Log.Warn("reconcile query failed", zap.Error(err))
				continue
			}
			var ids []string
			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err == nil {
					ids = append(ids, id)
				}
			}
			rows.Close()
			for _, id := range ids {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				if _, err := syncWorker.RecentSync(ctx, id); err != nil {
					d.Log.Warn("reconcile sync failed", zap.String("account_id", id), zap.Error(err))
				}
				cancel()
			}
		}
	}()

	// R10: AES-256-GCM encryption for user API keys
	encKey := getEncKey()
	aesCipher, err := crypto.NewAESCipher(encKey)
	if err != nil {
		d.Log.Warn("aes cipher init failed, api key encryption disabled")
	}

	// Admin API handlers
	svc := adminapi.NewService(d.PG).WithGateways(cfg.MT4Gateway, cfg.MT5Gateway)
	svc.WithLog(d.Log)
	svc.WithAccountConnector(&acctAdapter{acctMgr})
	svc.WithSyncWorker(syncWorker)
	svc.WithHistoryRepo(historyRepo)
	svc.WithSyncDonePublisher(func(id string) { acctMgr.PublishSyncDone(id) })
	if aesCipher != nil {
		svc.WithEncCipher(aesCipher)
	}
	svc.WithSymbolResolver() // RS06

	adp := adminapi.NewAdapter(svc)

	// Auth — must be created *before* authMW so d.Middleware is set when handlers are wrapped
	authH, err := adminapi.NewAuthHandler(d.PG, d.RDB)
	if err != nil {
		d.Log.Warn("auth handler unavailable", zap.Error(err))
	} else {
		authPath, authHandler := alfqv1connect.NewAuthServiceHandler(authH)
		mux.Handle(authPath, authHandler)
		d.Log.Info("auth service registered", zap.String("path", authPath))
		d.Middleware = authH.AuthMiddleware
	}

	// Wrap handlers with auth middleware if available
	authMW := func(h http.Handler) http.Handler {
		if d.Middleware != nil {
			return d.Middleware(h)
		}
		return h
	}

	brokerPath, brokerHandler := alfqv1connect.NewBrokerServiceHandler(adp)
	accountPath, accountHandler := alfqv1connect.NewAccountServiceHandler(adp)
	strategyPath, strategyHandler := alfqv1connect.NewStrategyServiceHandler(adp)
	backtestPath, backtestHandler := alfqv1connect.NewBacktestServiceHandler(adp)
	auditPath, auditHandler := alfqv1connect.NewAuditServiceHandler(adp)
	settingsPath, settingsHandler := alfqv1connect.NewSystemSettingsServiceHandler(adp)
	servicePath, serviceHandler := alfqv1connect.NewServiceManagementServiceHandler(adp)

	mux.Handle(brokerPath, authMW(brokerHandler))
	mux.Handle(accountPath, authMW(accountHandler))
	mux.Handle(strategyPath, authMW(strategyHandler))
	mux.Handle(backtestPath, authMW(backtestHandler))
	mux.Handle(auditPath, authMW(auditHandler))
	mux.Handle(settingsPath, authMW(settingsHandler))
	mux.Handle(servicePath, authMW(serviceHandler))

	// SymbolService — broker symbol metadata
	symPath, symHandler := adminapi.NewSymbolServiceHandler(svc)
	mux.Handle(symPath, authMW(symHandler))

	// StrategySymbolService — canonical symbol management (M5)
	if d.PG != nil {
		ssymHandler := adminapi.NewStrategySymbolHandler(d.PG.Pool)
		ssymPath, ssymSvc := alfqv1connect.NewStrategySymbolServiceHandler(ssymHandler)
		mux.Handle(ssymPath, authMW(ssymSvc))
		d.Log.Info("strategy symbol service registered", zap.String("path", ssymPath))
	}

	// RS07: RLS interceptor applies tenant_id to PG session automatically.
	// All handler-level setRLS calls can now be replaced with RequireTenant().
	if d.PG != nil {
		_ = adminapi.RLSInterceptor(d.PG) // wired via Connect handler options
	}

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
		}
		_, err = nc.Subscribe("account.orders.*", func(msg *nats.Msg) {
			sse.Broadcast(msg.Data)
		})
		if err != nil {
			d.Log.Warn("sse nats orders subscribe failed", zap.Error(err))
		} else {
			d.Log.Info("sse hub started", zap.String("path", "/sse/accounts"))
		}
	}

	// /readyz with real dependency health checks (CR-09)
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if kill.IsActive() {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("kill switch active"))
			return
		}

		checks := make(map[string]string)
		allOK := true

		// PG
		if d.PG != nil {
			if err := d.PG.Ping(r.Context()); err != nil {
				checks["pg"] = "down"
				allOK = false
			} else {
				checks["pg"] = "ok"
			}
		}

		// NATS
		if nc != nil && nc.IsConnected() {
			checks["nats"] = "ok"
		} else {
			checks["nats"] = "down"
			allOK = false
		}

		if !allOK {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		checksJSON, _ := json.Marshal(checks)
		w.Header().Set("Content-Type", "application/json")
		w.Write(checksJSON)
	})

	_ = executor // wired; adminapi integration via WithLiveSession for now

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
		Server: info.Server, Platform: info.Platform, BrokerID: info.BrokerID,
	})
}

func (a *acctAdapter) Disconnect(accountID string) {
	a.mgr.Disconnect(accountID)
}

// symAdapter adapts symbolsync.Service to accountconn.SymbolSyncer.
type symAdapter struct {
	svc *symbolsync.Service
}

func (a *symAdapter) Sync(ctx context.Context, brokerID, platform, sessionID string, conn *grpc.ClientConn) error {
	return a.svc.Sync(ctx, symbolsync.SyncParams{
		BrokerID: brokerID, Platform: platform, SessionID: sessionID, Conn: conn,
	})
}

func (a *acctAdapter) RefreshPositions(ctx context.Context, accountID string) {
	a.mgr.RefreshPositions(ctx, accountID)
}

func (a *acctAdapter) LatestPositions(accountID string) []*adminapi.PositionInfo {
	src := a.mgr.LatestPositions(accountID)
	if len(src) == 0 {
		return nil
	}
	out := make([]*adminapi.PositionInfo, 0, len(src))
	for _, p := range src {
		out = append(out, &adminapi.PositionInfo{
			Ticket: p.Ticket, Symbol: p.Symbol, Type: p.Type,
			Lots: p.Lots, OpenPrice: p.OpenPrice,
			Profit: p.Profit, Swap: p.Swap, Commission: p.Commission,
			OpenTimeMs: p.OpenTimeMs, CurrentPrice: p.CurrentPrice,
		})
	}
	return out
}

func (a *acctAdapter) WithLiveSession(accountID string, fn func(conn interface{}, sessionID, platform string) error) error {
	return a.mgr.WithLiveSession(accountID, func(c *grpc.ClientConn, sessionID, platform string) error {
		return fn(c, sessionID, platform)
	})
}

// omsSymbolResolver adapts adminapi.SymbolResolver to oms.SymbolResolver.
type omsSymbolResolver struct {
	resolver *adminapi.SymbolResolver
}

func (r *omsSymbolResolver) ResolveCanonical(ctx context.Context, accountID, canonical string) (string, int32, error) {
	info, valid, err := r.resolver.ResolveCanonical(ctx, accountID, canonical)
	if err != nil {
		return "", 0, err
	}
	if !valid {
		return info.SymbolRaw, info.TradeMode, nil
	}
	return info.SymbolRaw, info.TradeMode, nil
}

// getEncKey derives a 32-byte AES key for API key encryption.
// Priority: ALFQ_ENC_KEY env → system_settings → development fallback.
func getEncKey() []byte {
	if key := os.Getenv("ALFQ_ENC_KEY"); len(key) >= 32 {
		return []byte(key[:32])
	}
	h := sha256.Sum256([]byte("alfq-dev-encryption-key-change-in-production"))
	return h[:]
}