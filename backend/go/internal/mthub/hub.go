package mthub

import (
	"sync"

	"go.uber.org/zap"
)

// Hub manages the registry of MT sessions keyed by account ID.
type Hub struct {
	mu        sync.RWMutex
	sessions  map[string]*Session
	lookupGW  func(brokerID string) (Gateway, bool)
	log       *zap.Logger
}

// NewHub creates a Hub. lookupGW resolves a broker ID to a connected Gateway.
func NewHub(lookupGW func(brokerID string) (Gateway, bool), log *zap.Logger) *Hub {
	return &Hub{
		sessions: make(map[string]*Session),
		lookupGW: lookupGW,
		log:      log,
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
