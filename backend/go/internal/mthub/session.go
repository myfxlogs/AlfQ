// Package mthub — MT Session Hub: centralises MT4/MT5 session management
// behind the alfq.mthub.v1.MtHubService RPC.
package mthub

import (
	"sync"

	"google.golang.org/grpc"
)

// Gateway is the minimal interface mthub needs from an MT connection.
type Gateway interface {
	Platform() string
	Conn() *grpc.ClientConn
	SessionID() string
	BrokerID() string
}

// Session wraps a single Gateway with per-account metadata.
type Session struct {
	AccountID string
	Gateway   Gateway
	mu        sync.Mutex
}

// Platform returns the session platform ("mt4" or "mt5").
func (s *Session) Platform() string { return s.Gateway.Platform() }

// Conn returns the underlying gRPC connection (may be nil).
func (s *Session) Conn() *grpc.ClientConn {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Gateway.Conn()
}

// SessionID returns the MT session token (empty before Connect).
func (s *Session) SessionID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Gateway.SessionID()
}

// BrokerID returns the broker UUID for this session.
func (s *Session) BrokerID() string { return s.Gateway.BrokerID() }
