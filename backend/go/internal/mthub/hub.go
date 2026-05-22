package mthub

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Hub manages the registry of MT sessions keyed by account ID.
type Hub struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	lookupGW func(brokerID string) (Gateway, bool)
	pg       *pgxpool.Pool
	log      *zap.Logger
	prices   map[string]pricePair // symbol → latest bid+ask
}

type pricePair struct {
	Bid float64
	Ask float64
}

// UpdatePrice records the latest bid/ask for a symbol from a tick.
func (h *Hub) UpdatePrice(symbol string, bid, ask float64) {
	h.mu.Lock()
	if h.prices == nil {
		h.prices = make(map[string]pricePair)
	}
	h.prices[symbol] = pricePair{Bid: bid, Ask: ask}
	h.mu.Unlock()
}

// LatestBid returns the most recent bid.
func (h *Hub) LatestBid(symbol string) float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.prices[symbol].Bid
}

// LatestAsk returns the most recent ask.
func (h *Hub) LatestAsk(symbol string) float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.prices[symbol].Ask
}

// LatestPrice returns bid for buy positions, ask for sell positions.
func (h *Hub) LatestPriceForSide(symbol, side string) float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	p := h.prices[symbol]
	if side == "sell" {
		if p.Ask > 0 {
			return p.Ask
		}
	}
	return p.Bid
}

// NewHub creates a Hub. lookupGW resolves a broker ID to a connected Gateway.
// pg is optional; if non-nil, EnsureSession resolves accountID→brokerID via PG when needed.
func NewHub(lookupGW func(brokerID string) (Gateway, bool), pg *pgxpool.Pool, log *zap.Logger) *Hub {
	return &Hub{
		sessions: make(map[string]*Session),
		lookupGW: lookupGW,
		log:      log,
		pg:       pg,
	}
}

// EnsureSession finds or registers a session for the given account.
func (h *Hub) EnsureSession(accountID, brokerID string) (*Session, error) {
	h.mu.RLock()
	if s, ok := h.sessions[accountID]; ok {
		h.mu.RUnlock()
		return s, nil
	}
	h.mu.RUnlock()

	gw, ok := h.lookupGW(brokerID)
	if !ok && h.pg != nil {
		// Resolve broker_id from account_id via PG
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		var resolvedBrokerID string
		err := h.pg.QueryRow(ctx,
			`SELECT broker_id::text FROM accounts WHERE id = $1`, accountID,
		).Scan(&resolvedBrokerID)
		if err == nil && resolvedBrokerID != "" {
			gw, ok = h.lookupGW(resolvedBrokerID)
			if ok {
				h.log.Debug("mthub: resolved broker_id from account",
					zap.String("account_id", accountID),
					zap.String("broker_id", resolvedBrokerID),
				)
			}
		} else {
			h.log.Warn("mthub: failed to resolve broker_id",
				zap.String("account_id", accountID),
				zap.Error(err),
			)
		}
	}

	if !ok || gw == nil {
		h.log.Warn("mthub: no gateway for broker",
			zap.String("account_id", accountID),
			zap.String("broker_id", brokerID),
		)
		return nil, nil
	}

	s := &Session{AccountID: accountID, Gateway: gw}
	h.mu.Lock()
	h.sessions[accountID] = s
	h.mu.Unlock()

	h.log.Info("mthub: session registered",
		zap.String("account_id", accountID),
		zap.String("platform", gw.Platform()),
	)
	recordActiveSessions(h.ActiveSessions())
	return s, nil
}

// CloseSession removes a session from the registry.
func (h *Hub) CloseSession(accountID string) {
	h.mu.Lock()
	delete(h.sessions, accountID)
	h.mu.Unlock()
	recordActiveSessions(h.ActiveSessions())
}

// SessionCount returns the number of active sessions.
func (h *Hub) SessionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.sessions)
}

// ActiveSessions returns a snapshot of active session IDs grouped by platform.
func (h *Hub) ActiveSessions() map[string]int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make(map[string]int)
	for _, s := range h.sessions {
		out[s.Platform()]++
	}
	return out
}