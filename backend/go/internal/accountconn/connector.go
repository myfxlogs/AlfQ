// Package accountconn manages persistent MT4/MT5 connections for bound trading accounts.
// Event-driven: subscribes to OnOrderProfit gRPC stream for real-time balance/equity,
// with periodic AccountSummary for full details.
package accountconn

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/alfq/backend/go/internal/common/config"
	"github.com/alfq/backend/go/internal/common/db/pg"
	"github.com/alfq/backend/go/internal/mthub"
	"github.com/alfq/backend/go/internal/oms/repo"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	mt4pb "github.com/alfq/backend/go/gen/mt4"
	mt5pb "github.com/alfq/backend/go/gen/mt5"
	"github.com/google/uuid"
)

// AccountInfo holds data needed to maintain a connection.
type AccountInfo struct {
	ID       string
	Login    string
	Password string
	Server   string
	Platform string // "MT4" or "MT5"
	BrokerID string // UUID of broker record
}

// Manager manages persistent MT connections per account.
type Manager struct {
	mu          sync.Mutex
	sessions    map[string]*session
	log         *zap.Logger
	pool        *pg.Pool
	rdb         redis.UniversalClient
	nc          *nats.Conn
	js          nats.JetStreamContext
	mt4gw       config.GatewayConfig
	mt5gw       config.GatewayConfig
	symSvc      SymbolSyncer
	closed      bool
	syncWorker  *SyncWorker
	mthubClient *mthub.Client

	// onAccountUpdate is called when balance/equity/margin changes.
	// Set via SetOnAccountUpdate to wire risk engine state updates.
	onAccountUpdate func(accountID string, balance, equity, margin, freeMargin float64)
}

type session struct {
	info   AccountInfo
	cancel context.CancelFunc

	// live connection state, updated by streamLoop. Nil until the gateway
	// connection is established. Protected by liveMu.
	liveMu       sync.RWMutex
	liveConn     *grpc.ClientConn
	liveSession  string
	livePosition []*PositionInfo
}

func (s *session) setLive(conn *grpc.ClientConn, sessionID string) {
	s.liveMu.Lock()
	s.liveConn = conn
	s.liveSession = sessionID
	s.liveMu.Unlock()
}

func (s *session) clearLive() {
	s.liveMu.Lock()
	s.liveConn = nil
	s.liveSession = ""
	s.liveMu.Unlock()
}

func (s *session) setPositions(p []*PositionInfo) {
	s.liveMu.Lock()
	s.livePosition = p
	s.liveMu.Unlock()
}

func (s *session) getLive() (*grpc.ClientConn, string, string) {
	s.liveMu.RLock()
	defer s.liveMu.RUnlock()
	return s.liveConn, s.liveSession, s.info.Platform
}

func (s *session) getPositions() []*PositionInfo {
	s.liveMu.RLock()
	defer s.liveMu.RUnlock()
	return s.livePosition
}

// LatestPositions returns the most-recently fetched positions for the account.
// Returns nil if the account has no live session or no positions have been fetched yet.
// RefreshPositions fetches fresh positions from the broker via mthub (RS03).
func (m *Manager) RefreshPositions(ctx context.Context, accountID string) {
	positions := fetchPositionsViaMthub(ctx, m.mthubClient, accountID)
	if len(positions) > 0 {
		m.mu.Lock()
		if s, ok := m.sessions[accountID]; ok {
			s.setPositions(positions)
		}
		m.mu.Unlock()
	}
}

func (m *Manager) LatestPositions(accountID string) []*PositionInfo {
	m.mu.Lock()
	s, ok := m.sessions[accountID]
	m.mu.Unlock()
	if !ok {
		return nil
	}
	return s.getPositions()
}

// WithLiveSession calls fn with the live gateway connection and session ID for
// an account. Returns an error if no live session is available.
func (m *Manager) WithLiveSession(accountID string, fn func(conn *grpc.ClientConn, sessionID, platform string) error) error {
	m.mu.Lock()
	s, ok := m.sessions[accountID]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("account %s has no active connection", accountID)
	}
	conn, sessionID, platform := s.getLive()
	if conn == nil || sessionID == "" {
		return fmt.Errorf("account %s connection is not ready", accountID)
	}
	return fn(conn, sessionID, platform)
}

// SymbolSyncer abstracts symbol sync triggers.
type SymbolSyncer interface {
	Sync(ctx context.Context, brokerID, platform, sessionID string, conn *grpc.ClientConn) error
}

// NewManager creates an account connection manager.
func NewManager(log *zap.Logger, pool *pg.Pool, rdb redis.UniversalClient, nc *nats.Conn, js nats.JetStreamContext,
	mt4gw, mt5gw config.GatewayConfig, symSvc SymbolSyncer, mthubAddr string) *Manager {
	return &Manager{
		sessions:    make(map[string]*session),
		log:         log,
		pool:        pool,
		rdb:         rdb,
		nc:          nc,
		js:          js,
		mt4gw:       mt4gw,
		mt5gw:       mt5gw,
		symSvc:      symSvc,
		mthubClient: mthub.NewClient(mthubAddr),
	}
}

// SetOnAccountUpdate registers a callback invoked when account balance/equity/margin changes.
// Used by risk engine to maintain real-time AccountState for risk rule evaluation.
func (m *Manager) SetOnAccountUpdate(fn func(accountID string, balance, equity, margin, freeMargin float64)) {
	m.mu.Lock()
	m.onAccountUpdate = fn
	m.mu.Unlock()
}

// SetSyncWorker binds the sync worker for order history persistence.
func (m *Manager) SetSyncWorker(sw *SyncWorker) {
	m.syncWorker = sw
	if sw != nil {
		sw.SetManager(m)
	}
}

// Connect starts a persistent event-driven connection.
func (m *Manager) Connect(ctx context.Context, info AccountInfo) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	if s, ok := m.sessions[info.ID]; ok {
		s.cancel()
	}
	runCtx, cancel := context.WithCancel(context.Background())
	m.sessions[info.ID] = &session{info: info, cancel: cancel}
	m.mu.Unlock()

	go m.run(runCtx, info)
	m.log.Info("account connector started",
		zap.String("account_id", info.ID),
		zap.String("login", info.Login),
	)
}

// Disconnect stops the persistent connection.
func (m *Manager) Disconnect(accountID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[accountID]; ok {
		s.cancel()
		delete(m.sessions, accountID)
		m.log.Info("account connector stopped", zap.String("account_id", accountID))
	}
}

// Shutdown stops all connections.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	m.closed = true
	for id, s := range m.sessions {
		s.cancel()
		delete(m.sessions, id)
	}
	m.mu.Unlock()
	m.log.Info("account connector shutdown complete")
}

// ActiveCount returns active session count.
func (m *Manager) ActiveCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sessions)
}

// run: connect + stream OnOrderProfit + reconnect on failure.
func (m *Manager) run(ctx context.Context, info AccountInfo) {
	backoff := 1 * time.Second
	const maxBackoff = 60 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		gw := m.mt5gw
		if strings.EqualFold(info.Platform, "MT4") {
			gw = m.mt4gw
		}

		timeout := gw.Timeout
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		dialCtx, dialCancel := context.WithTimeout(ctx, timeout)
		var creds credentials.TransportCredentials
		if gw.UseTLS {
			creds = credentials.NewTLS(&tls.Config{})
		} else {
			creds = insecure.NewCredentials()
		}
		conn, err := grpc.DialContext(dialCtx, gw.Addr, //nolint:staticcheck
			grpc.WithTransportCredentials(creds),
			grpc.WithBlock(), //nolint:staticcheck
		)
		dialCancel()
		if err != nil {
			m.log.Warn("dial gateway failed",
				zap.String("account_id", info.ID),
				zap.Error(err),
			)
			m.updateStatus(ctx, info.ID, "error", err.Error())
			goto reconnect
		}

		if err := m.streamLoop(ctx, conn, info); err != nil {
			m.log.Warn("stream ended",
				zap.String("account_id", info.ID),
				zap.Error(err),
			)
			m.updateStatus(ctx, info.ID, "error", err.Error())
		}
		_ = conn.Close()

	reconnect:
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// streamLoop runs the connect+stream+poll cycle on an established gRPC connection.
func (m *Manager) streamLoop(ctx context.Context, conn *grpc.ClientConn, info AccountInfo) error {
	sessionID, err := m.mtConnect(ctx, conn, info)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	// Publish live session so other components (admin API) can reuse it.
	m.mu.Lock()
	sess := m.sessions[info.ID]
	m.mu.Unlock()
	if sess != nil {
		sess.setLive(conn, sessionID)
		defer sess.clearLive()
	}

	// Fetch initial full account summary + positions
	acct, err := fetchAccountSummary(ctx, conn, info.Platform, sessionID)
	if err != nil {
		m.log.Warn("initial summary failed",
			zap.String("account_id", info.ID),
			zap.Error(err),
		)
	} else {
		positions := fetchPositionsViaMthub(ctx, m.mthubClient, info.ID)
		if sess != nil {
			sess.setPositions(positions)
		}
		m.publishAndUpdate(ctx, info.ID, acct, positions)
	}

	// Reconcile: if account has never been synced (total_synced==0),
	// trigger a full sync first. Otherwise do incremental 5min window.
	// This runs on every (re)connection per the reconnect → IncrSync design.
	if m.syncWorker != nil {
		go func() {
			rctx, rcancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer rcancel()
			state, err := m.syncWorker.GetSyncState(rctx, info.ID)
			if err != nil || state == nil || state.TotalSynced == 0 {
				// First sync — do full
				if err := m.syncWorker.FullSync(rctx, info.ID); err != nil {
					m.log.Warn("initial full sync failed",
						zap.String("account_id", info.ID),
						zap.Error(err),
					)
				}
				// Push SSE so frontend knows to refresh order list
				m.publishSyncDone(info.ID)
			} else {
				// Already synced — do incremental reconciliation.
				// Uses last_incr_sync_at if available, otherwise default 5-min window.
				from := time.Now().Add(-5 * time.Minute)
				if state.LastIncrSyncAt != nil {
					from = state.LastIncrSyncAt.Add(-5 * time.Minute)
				}
				to := time.Now()
				m.log.Info("reconnect incrSync",
					zap.String("account_id", info.ID),
					zap.Time("from", from),
					zap.Time("to", to),
				)
				if _, err := m.syncWorker.IncrSync(rctx, info.ID, from, to); err != nil {
					m.log.Warn("reconnect incrSync failed",
						zap.String("account_id", info.ID),
						zap.Error(err),
					)
				}
			}
		}()
	}

	// Trigger symbol metadata sync (async, non-blocking), then periodic every 6h
	if m.symSvc != nil {
		go func() {
			if err := m.symSvc.Sync(context.Background(), info.BrokerID, info.Platform, sessionID, conn); err != nil {
				m.log.Warn("symbol sync failed",
					zap.String("account_id", info.ID),
					zap.Error(err),
				)
			}
			// Periodic refresh every 6 hours
			ticker := time.NewTicker(6 * time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if err := m.symSvc.Sync(context.Background(), info.BrokerID, info.Platform, sessionID, conn); err != nil {
						m.log.Warn("periodic symbol sync failed",
							zap.String("account_id", info.ID),
							zap.Error(err),
						)
					}
				}
			}
		}()
	}

	// Periodic order history reconciliation ticker (10 min, per design §5.10)
	if m.syncWorker != nil {
		go func() {
			ticker := time.NewTicker(10 * time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					sctx, scancel := context.WithTimeout(context.Background(), 30*time.Second)
					if _, err := m.syncWorker.RecentSync(sctx, info.ID); err != nil {
						m.log.Warn("periodic order sync failed",
							zap.String("account_id", info.ID),
							zap.Error(err),
						)
					}
					scancel()
				}
			}
		}()
	}

	// Try event-driven streaming first
	if err := m.eventLoop(ctx, conn, info, sessionID); err != nil {
		m.log.Warn("event stream failed, falling back to poll",
			zap.String("account_id", info.ID),
			zap.Error(err),
		)
		return m.pollLoop(ctx, conn, info, sessionID)
	}
	return nil
}

// eventLoop subscribes to OnOrderProfit stream.
func (m *Manager) eventLoop(ctx context.Context, conn *grpc.ClientConn, info AccountInfo, sessionID string) error {
	stream, err := subscribeProfitStream(ctx, conn, info.Platform, sessionID)
	if err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}

	// Background periodic full AccountSummary + positions (every 30s)
	fullPollDone := make(chan struct{})
	go func() {
		defer close(fullPollDone)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				acct, err := fetchAccountSummary(ctx, conn, info.Platform, sessionID)
				if err != nil {
					m.log.Warn("full poll failed",
						zap.String("account_id", info.ID),
						zap.Error(err),
					)
					continue
				}
				positions := fetchPositionsViaMthub(ctx, m.mthubClient, info.ID)
				m.mu.Lock()
				if s, ok := m.sessions[info.ID]; ok {
					s.setPositions(positions)
				}
				m.mu.Unlock()
				m.publishAndUpdate(ctx, info.ID, acct, positions)
			}
		}
	}()
	defer func() { <-fullPollDone }()

	// Background OnOrderUpdate listener — fires whenever an order is opened,
	// closed or modified on the server. We refresh positions+summary and also
	// run an incremental order sync (last 5 min) so local DB stays up to date.
	orderEvtDone := make(chan struct{})
	go func() {
		defer close(orderEvtDone)
		evtStream, err := subscribeOrderUpdateStream(ctx, conn, info.Platform, sessionID)
		if err != nil {
			m.log.Warn("subscribe order update stream failed",
				zap.String("account_id", info.ID),
				zap.Error(err),
			)
			return
		}
		for {
			if err := evtStream.Recv(); err != nil {
				if ctx.Err() == nil {
					m.log.Warn("order update stream ended",
						zap.String("account_id", info.ID),
						zap.Error(err),
					)
				}
				return
			}
			// Order event received — refresh positions+summary and broadcast.
			acct, err := fetchAccountSummary(ctx, conn, info.Platform, sessionID)
			if err != nil {
				m.log.Warn("post-order-event summary fetch failed",
					zap.String("account_id", info.ID),
					zap.Error(err),
				)
				continue
			}
			positions := fetchPositionsViaMthub(ctx, m.mthubClient, info.ID)
			m.mu.Lock()
			if s, ok := m.sessions[info.ID]; ok {
				s.setPositions(positions)
			}
			m.mu.Unlock()
			m.publishEvent(info.ID, acct, positions, true)
			m.updateDB(ctx, info.ID, acct)

			// Incremental order sync (last 5 min) + publish deltas
			if m.syncWorker != nil {
				go func() {
					sctx, scancel := context.WithTimeout(context.Background(), 20*time.Second)
					defer scancel()
					changed, err := m.syncWorker.RecentSync(sctx, info.ID)
					if err != nil {
						m.log.Warn("incremental sync failed",
							zap.String("account_id", info.ID),
							zap.Error(err),
						)
					} else if len(changed) > 0 {
						m.publishOrderDelta(info.ID, changed)
					}
				}()
			}
		}
	}()
	defer func() { <-orderEvtDone }()

	// Read stream events
	isMT5 := strings.EqualFold(info.Platform, "MT5")
	for {
		update, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("recv: %w", err)
		}

		result := update.GetResult()
		if result == nil {
			continue
		}

		// ── MT5 path ───────────────────────────────────────────────────────
		// MT5 OnOrderProfit stream's Profit/Equity/Balance fields do NOT
		// match the terminal ACCOUNT_PROFIT (MQL5 AccountInfoDouble). The
		// canonical source is unary AccountSummary (same family as terminal
		// snapshot). We trigger AccountSummary on each stream frame so that
		// the 30s background poller and the stream cannot disagree → no
		// floating-profit flapping between two values.
		// MT4 keeps the original eq-bal derivation (this issue is MT5-only).
		if isMT5 {
			sctx, cancel := context.WithTimeout(ctx, 3*time.Second)
			acct, aerr := fetchAccountSummary(sctx, conn, info.Platform, sessionID)
			cancel()
			if aerr == nil && acct != nil {
				// Mismatch monitoring: compare derived (eq-bal) vs terminal Profit.
				derived := result.GetEquity() - result.GetBalance()
				if math.Abs(derived-acct.Profit) > 1.0 {
					mt5ProfitMismatchTotal.Inc()
					if m.log != nil {
						m.log.Warn("mt5 profit mismatch (stream eq-bal vs AccountSummary.Profit)",
							zap.String("account_id", info.ID),
							zap.Float64("summary_profit", acct.Profit),
							zap.Float64("derived_eq_bal", derived),
							zap.Float64("summary_equity", acct.Equity),
							zap.Float64("summary_balance", acct.Balance),
						)
					}
				}
				m.publishAndUpdate(ctx, info.ID, acct, nil)
				if m.onAccountUpdate != nil {
					m.onAccountUpdate(info.ID, acct.Balance, acct.Equity,
						acct.Margin, acct.FreeMargin)
				}
				continue
			}
			if m.log != nil {
				m.log.Debug("mt5 AccountSummary in stream loop failed; falling back to stream frame",
					zap.String("account_id", info.ID),
					zap.Error(aerr),
				)
			}
			// fall through to MT4-style derivation as a degraded fallback
		}

		// ── MT4 path (and MT5 fallback when AccountSummary RPC failed) ─────
		// Stream only gives balance/equity; merge with current DB state to avoid zeroing other fields.
		// 浮动盈亏 = equity - balance (MT 公式：Equity = Balance + Floating Profit + Credit；
		// demo/常规账户 credit≈0)，由流数据派生，保证浮动盈亏与净值一同实时更新。
		bal := result.GetBalance()
		eq := result.GetEquity()
		partial := &AccountSummaryInfo{
			Balance: bal,
			Equity:  eq,
			Profit:  eq - bal,
		}
		// Read current DB values for fields not in the stream
		var margin, freeMargin, marginLevel, leverage float64
		var currency string
		dbErr := m.pool.QueryRow(ctx,
			`SELECT COALESCE(margin,0), COALESCE(free_margin,0), COALESCE(margin_level,0),
			        COALESCE(currency,''), COALESCE(leverage,0)
			 FROM accounts WHERE id=$1`, info.ID,
		).Scan(&margin, &freeMargin, &marginLevel, &currency, &leverage)
		if dbErr == nil {
			partial.Margin = margin
			partial.FreeMargin = freeMargin
			partial.MarginLevel = marginLevel
			partial.Currency = currency
			partial.Leverage = int32(leverage)
		}
		m.publishAndUpdate(ctx, info.ID, partial, nil)
		// Push account state to risk engine for real-time rule evaluation
		if m.onAccountUpdate != nil && partial != nil {
			m.onAccountUpdate(info.ID, partial.Balance, partial.Equity,
				partial.Margin, partial.FreeMargin)
		}
	}
}

// pollLoop fallback: periodic full AccountSummary + positions via the same connection.
func (m *Manager) pollLoop(ctx context.Context, conn *grpc.ClientConn, info AccountInfo, sessionID string) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			acct, err := fetchAccountSummary(ctx, conn, info.Platform, sessionID)
			if err != nil {
				return fmt.Errorf("poll: %w", err)
			}
			positions := fetchPositionsViaMthub(ctx, m.mthubClient, info.ID)
			m.mu.Lock()
			if s, ok := m.sessions[info.ID]; ok {
				s.setPositions(positions)
			}
			m.mu.Unlock()
			m.publishAndUpdate(ctx, info.ID, acct, positions)
		}
	}
}

// mtConnect authenticates to the MT server and returns a session ID.
func (m *Manager) mtConnect(ctx context.Context, conn *grpc.ClientConn, info AccountInfo) (string, error) {
	tempID := uuid.New().String()
	host, port := splitHostPort(info.Server, "443")

	switch strings.ToUpper(info.Platform) {
	case "MT5":
		connClient := mt5pb.NewConnectionClient(conn)
		ctxWithID := metadata.AppendToOutgoingContext(ctx, "id", tempID)
		resp, err := connClient.Connect(ctxWithID, &mt5pb.ConnectRequest{
			User:     parseUint(info.Login),
			Password: info.Password,
			Host:     host,
			Port:     int32(atoi(port)),
		})
		if err != nil {
			return "", fmt.Errorf("mt5: %w", err)
		}
		if e := resp.GetError(); e != nil && e.GetMessage() != "" {
			return "", fmt.Errorf("mt5: %s", e.GetMessage())
		}
		return resp.GetResult(), nil

	case "MT4":
		connClient := mt4pb.NewConnectionClient(conn)
		ctxWithID := metadata.AppendToOutgoingContext(ctx, "id", tempID)
		resp, err := connClient.Connect(ctxWithID, &mt4pb.ConnectRequest{
			User:     int32(parseUint(info.Login)),
			Password: info.Password,
			Host:     host,
			Port:     int32(atoi(port)),
			Id:       &tempID,
		})
		if err != nil {
			return "", fmt.Errorf("mt4: %w", err)
		}
		if e := resp.GetError(); e != nil && e.GetMessage() != "" {
			return "", fmt.Errorf("mt4: %s", e.GetMessage())
		}
		return resp.GetResult(), nil

	default:
		return "", fmt.Errorf("unknown platform: %s", info.Platform)
	}
}

// ── Stream adapters ──

type profitStream interface {
	Recv() (profitUpdate, error)
}
type profitUpdate interface {
	GetResult() profitResult
}
type profitResult interface {
	GetBalance() float64
	GetEquity() float64
}

func subscribeProfitStream(ctx context.Context, conn *grpc.ClientConn, platform, sessionID string) (profitStream, error) {
	ctxWithID := metadata.AppendToOutgoingContext(ctx, "id", sessionID)
	switch strings.ToUpper(platform) {
	case "MT5":
		client := mt5pb.NewStreamsClient(conn)
		s, err := client.OnOrderProfit(ctxWithID, &mt5pb.OnOrderProfitRequest{Id: sessionID})
		if err != nil {
			return nil, err
		}
		return &mt5Stream{s: s}, nil
	case "MT4":
		client := mt4pb.NewStreamsClient(conn)
		s, err := client.OnOrderProfit(ctxWithID, &mt4pb.OnOrderProfitRequest{Id: sessionID})
		if err != nil {
			return nil, err
		}
		return &mt4Stream{s: s}, nil
	}
	return nil, fmt.Errorf("unknown platform: %s", platform)
}

type mt5Stream struct {
	s mt5pb.Streams_OnOrderProfitClient
}
type mt5Upd struct{ *mt5pb.OnOrderProfitReply }
type mt5Res struct{ *mt5pb.ProfitUpdate }

func (s *mt5Stream) Recv() (profitUpdate, error) {
	m, e := s.s.Recv()
	if e != nil {
		return nil, e
	}
	return &mt5Upd{m}, nil
}
func (u *mt5Upd) GetResult() profitResult {
	if r := u.OnOrderProfitReply.GetResult(); r != nil {
		return &mt5Res{r}
	}
	return nil
}
func (r *mt5Res) GetBalance() float64 { return r.ProfitUpdate.GetBalance() }
func (r *mt5Res) GetEquity() float64  { return r.ProfitUpdate.GetEquity() }

type mt4Stream struct {
	s mt4pb.Streams_OnOrderProfitClient
}
type mt4Upd struct{ *mt4pb.OnOrderProfitReply }
type mt4Res struct{ *mt4pb.ProfitUpdate }

func (s *mt4Stream) Recv() (profitUpdate, error) {
	m, e := s.s.Recv()
	if e != nil {
		return nil, e
	}
	return &mt4Upd{m}, nil
}
func (u *mt4Upd) GetResult() profitResult {
	if r := u.OnOrderProfitReply.GetResult(); r != nil {
		return &mt4Res{r}
	}
	return nil
}
func (r *mt4Res) GetBalance() float64 { return r.ProfitUpdate.GetBalance() }
func (r *mt4Res) GetEquity() float64  { return r.ProfitUpdate.GetEquity() }

// orderUpdateStream is a minimal abstraction over MT4/MT5 OnOrderUpdate streams.
// We don't need the event payload — its arrival alone signals that positions
// and history should be refreshed.
type orderUpdateStream interface {
	Recv() error
}

func subscribeOrderUpdateStream(ctx context.Context, conn *grpc.ClientConn, platform, sessionID string) (orderUpdateStream, error) {
	ctxWithID := metadata.AppendToOutgoingContext(ctx, "id", sessionID)
	switch strings.ToUpper(platform) {
	case "MT5":
		client := mt5pb.NewStreamsClient(conn)
		s, err := client.OnOrderUpdate(ctxWithID, &mt5pb.OnOrderUpdateRequest{Id: sessionID})
		if err != nil {
			return nil, err
		}
		return &mt5OrderStream{s: s}, nil
	case "MT4":
		client := mt4pb.NewStreamsClient(conn)
		s, err := client.OnOrderUpdate(ctxWithID, &mt4pb.OnOrderUpdateRequest{Id: sessionID})
		if err != nil {
			return nil, err
		}
		return &mt4OrderStream{s: s}, nil
	}
	return nil, fmt.Errorf("unknown platform: %s", platform)
}

type mt5OrderStream struct {
	s mt5pb.Streams_OnOrderUpdateClient
}

func (s *mt5OrderStream) Recv() error { _, e := s.s.Recv(); return e }

type mt4OrderStream struct {
	s mt4pb.Streams_OnOrderUpdateClient
}

func (s *mt4OrderStream) Recv() error { _, e := s.s.Recv(); return e }

// ── Helpers ──

func (m *Manager) publishAndUpdate(ctx context.Context, accountID string, info *AccountSummaryInfo, positions []*PositionInfo) {
	m.publish(accountID, info, positions)
	m.updateDB(ctx, accountID, info)
}

func (m *Manager) publish(accountID string, info *AccountSummaryInfo, positions []*PositionInfo) {
	m.publishEvent(accountID, info, positions, false)
}

func (m *Manager) publishEvent(accountID string, info *AccountSummaryInfo, positions []*PositionInfo, orderEvent bool) {
	if m.nc == nil {
		return
	}
	subject := fmt.Sprintf("account.status.%s", accountID)

	type posOut struct {
		Ticket       int64   `json:"ticket"`
		Symbol       string  `json:"symbol"`
		Type         string  `json:"type"`
		Lots         float64 `json:"lots"`
		OpenPrice    float64 `json:"openPrice"`
		Profit       float64 `json:"profit"`
		Swap         float64 `json:"swap"`
		Commission   float64 `json:"commission"`
		OpenTimeMs   int64   `json:"openTimeMs"`
		CurrentPrice float64 `json:"currentPrice"`
	}

	payload := map[string]interface{}{
		"accountId":   accountID,
		"balance":     info.Balance,
		"equity":      info.Equity,
		"margin":      info.Margin,
		"freeMargin":  info.FreeMargin,
		"marginLevel": info.MarginLevel,
		"profit":      info.Profit,
		"currency":    info.Currency,
		"leverage":    info.Leverage,
	}
	if positions != nil {
		out := make([]posOut, 0, len(positions))
		for _, p := range positions {
			out = append(out, posOut{
				Ticket: p.Ticket, Symbol: p.Symbol, Type: p.Type,
				Lots: p.Lots, OpenPrice: p.OpenPrice,
				Profit: p.Profit, Swap: p.Swap, Commission: p.Commission,
				OpenTimeMs: p.OpenTimeMs, CurrentPrice: p.CurrentPrice,
			})
		}
		payload["positions"] = out
	}
	if orderEvent {
		payload["orderEvent"] = true
	}

	data, _ := json.Marshal(payload)
	_ = m.nc.Publish(subject, data)
}

// PublishSyncDone pushes a sync-complete SSE event for the given account.
func (m *Manager) PublishSyncDone(accountID string) {
	m.publishSyncDone(accountID)
}

func (m *Manager) publishSyncDone(accountID string) {
	if m.nc == nil {
		return
	}
	payload := map[string]interface{}{
		"accountId": accountID,
		"type":      "order_sync_done",
	}
	data, _ := json.Marshal(payload)
	subj := fmt.Sprintf("account.orders.%s", accountID)
	_ = m.nc.Publish(subj, data)
}

func (m *Manager) publishOrderDelta(accountID string, changed []*repo.HistoryOrder) {
	if m.nc == nil || len(changed) == 0 {
		return
	}
	changes := make([]map[string]interface{}, 0, len(changed))
	for _, o := range changed {
		openTime := o.OpenTime.Format(time.RFC3339)
		var closeTime string
		if o.CloseTime != nil {
			closeTime = o.CloseTime.Format(time.RFC3339)
		}
		changes = append(changes, map[string]interface{}{
			"op": "upsert",
			"order": map[string]interface{}{
				"ticket":     o.Ticket,
				"symbol":     o.Symbol,
				"side":       o.Side,
				"lots":       o.Lots,
				"openPrice":  o.OpenPrice,
				"closePrice": o.ClosePrice,
				"profit":     o.Profit,
				"swap":       o.Swap,
				"commission": o.Commission,
				"openTime":   openTime,
				"closeTime":  closeTime,
				"state":      o.State,
			},
		})
	}
	subject := fmt.Sprintf("account.orders.%s", accountID)
	payload := map[string]interface{}{
		"accountId": accountID,
		"type":      "order_delta",
		"changes":   changes,
	}
	data, _ := json.Marshal(payload)
	_ = m.nc.Publish(subject, data)
}

func (m *Manager) updateDB(ctx context.Context, accountID string, info *AccountSummaryInfo) {
	_, _ = m.pool.Exec(ctx, `
		UPDATE accounts
		SET status='connected', balance=$1, equity=$2, margin=$3,
		    free_margin=$4, margin_level=$5, profit=$6,
		    currency=$7, leverage=$8, connected_at=now(), updated_at=now(),
		    account_type = CASE WHEN $10 != '' AND account_type != $10 THEN $10 ELSE account_type END
		WHERE id=$9
	`, info.Balance, info.Equity, info.Margin, info.FreeMargin,
		info.MarginLevel, info.Profit, info.Currency, info.Leverage, accountID, info.AccountType)

	// Cache in Redis with 2min TTL
	if m.rdb != nil {
		key := "alfq:account:" + accountID
		data := fmt.Sprintf(`{"balance":%.2f,"equity":%.2f,"margin":%.2f,"freeMargin":%.2f,"marginLevel":%.2f,"profit":%.2f,"currency":"%s","leverage":%d}`,
			info.Balance, info.Equity, info.Margin, info.FreeMargin,
			info.MarginLevel, info.Profit, info.Currency, info.Leverage,
		)
		m.rdb.Set(ctx, key, data, 120*time.Second)
	}
}

func (m *Manager) updateStatus(ctx context.Context, accountID, status, lastError string) {
	_, _ = m.pool.Exec(ctx, `
		UPDATE accounts SET status=$1, last_error=$2, updated_at=now() WHERE id=$3
	`, status, lastError, accountID)
}

func splitHostPort(hostPort, defaultPort string) (string, string) {
	for i := len(hostPort) - 1; i >= 0; i-- {
		if hostPort[i] == ':' {
			return hostPort[:i], hostPort[i+1:]
		}
	}
	return hostPort, defaultPort
}

func parseUint(s string) uint64 {
	var n uint64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + uint64(c-'0')
		}
	}
	return n
}

func atoi(s string) int {
	var n int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}

// fetchPositionsViaMthub fetches opened orders through the MT Session Hub.
func fetchPositionsViaMthub(ctx context.Context, client *mthub.Client, accountID string) []*PositionInfo {
	if client == nil {
		return nil
	}
	orders, err := client.OpenedOrders(ctx, accountID)
	if err != nil {
		log.Printf("mthub OpenedOrders failed for %s: %v", accountID, err)
		return nil
	}
	log.Printf("mthub OpenedOrders for %s: %d positions", accountID, len(orders))
	if len(orders) > 0 {
		log.Printf("DEBUG first order: ticket=%d openTimeMs=%d currentPrice=%.5f", orders[0].Ticket, orders[0].OpenTimeMs, orders[0].CurrentPrice)
	}
	out := make([]*PositionInfo, 0, len(orders))
	for _, o := range orders {
		out = append(out, &PositionInfo{
			Ticket: o.Ticket, Symbol: o.Symbol, Type: o.Side, Lots: o.Lots,
			OpenPrice: o.OpenPrice, Profit: o.Profit, Swap: o.Swap, Commission: o.Commission,
			OpenTimeMs: o.OpenTimeMs, CurrentPrice: o.CurrentPrice,
		})
	}
	return out
}

// fetchAccountSummary fetches the full account summary using MT gRPC calls
// on an existing connection (formerly via external mtapi package).
func fetchAccountSummary(ctx context.Context, conn *grpc.ClientConn, platform, sessionID string) (*AccountSummaryInfo, error) {
	ctxWithID := metadata.AppendToOutgoingContext(ctx, "id", sessionID)
	switch strings.ToUpper(platform) {
	case "MT5":
		client := mt5pb.NewMT5Client(conn)
		resp, err := client.AccountSummary(ctxWithID, &mt5pb.AccountSummaryRequest{Id: sessionID})
		if err != nil {
			return nil, fmt.Errorf("mt5 account summary: %w", err)
		}
		summ := resp.GetResult()
		if summ == nil {
			return &AccountSummaryInfo{}, nil
		}
		return &AccountSummaryInfo{
			Balance:     summ.GetBalance(),
			Equity:      summ.GetEquity(),
			Margin:      summ.GetMargin(),
			FreeMargin:  summ.GetFreeMargin(),
			MarginLevel: summ.GetMarginLevel(),
			Profit:      summ.GetProfit(),
			Currency:    summ.GetCurrency(),
			Leverage:    int32(summ.GetLeverage()),
		}, nil
	case "MT4":
		client := mt4pb.NewMT4Client(conn)
		resp, err := client.AccountSummary(ctxWithID, &mt4pb.AccountSummaryRequest{Id: sessionID})
		if err != nil {
			return nil, fmt.Errorf("mt4 account summary: %w", err)
		}
		summ := resp.GetResult()
		if summ == nil {
			return &AccountSummaryInfo{}, nil
		}
		return &AccountSummaryInfo{
			Balance:     summ.GetBalance(),
			Equity:      summ.GetEquity(),
			Margin:      summ.GetMargin(),
			FreeMargin:  summ.GetFreeMargin(),
			MarginLevel: summ.GetMarginLevel(),
			Profit:      summ.GetProfit(),
			Currency:    summ.GetCurrency(),
			Leverage:    int32(summ.GetLeverage()),
		}, nil
	default:
		return nil, fmt.Errorf("unknown platform: %s", platform)
	}
}
