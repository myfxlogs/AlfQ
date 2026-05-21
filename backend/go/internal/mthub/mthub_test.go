package mthub

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	mthubv1 "github.com/alfq/backend/go/gen/alfq/mthub/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// mockGateway implements Gateway for testing.
type mockGateway struct {
	platform  string
	conn      *grpc.ClientConn
	sessionID string
	brokerID  string
}

func (m *mockGateway) Platform() string       { return m.platform }
func (m *mockGateway) Conn() *grpc.ClientConn { return m.conn }
func (m *mockGateway) SessionID() string      { return m.sessionID }
func (m *mockGateway) BrokerID() string       { return m.brokerID }

func TestSession(t *testing.T) {
	gw := &mockGateway{platform: "mt5", sessionID: "sess-1", brokerID: "broker-1"}
	s := &Session{AccountID: "acc-1", Gateway: gw}

	if s.Platform() != "mt5" {
		t.Errorf("Platform = %s, want mt5", s.Platform())
	}
	if s.SessionID() != "sess-1" {
		t.Errorf("SessionID = %s, want sess-1", s.SessionID())
	}
	if s.BrokerID() != "broker-1" {
		t.Errorf("BrokerID = %s, want broker-1", s.BrokerID())
	}
}

func TestHubEnsureSession(t *testing.T) {
	gw := &mockGateway{platform: "mt5", sessionID: "sess-1", brokerID: "b1"}
	lookup := func(brokerID string) (Gateway, bool) {
		if brokerID == "b1" {
			return gw, true
		}
		return nil, false
	}
	hub := NewHub(lookup, zap.NewNop())

	// First call: should register session.
	ses, err := hub.EnsureSession("acc-1", "b1")
	if err != nil {
		t.Fatal(err)
	}
	if ses == nil {
		t.Fatal("expected non-nil session")
	}
	if ses.Platform() != "mt5" {
		t.Errorf("Platform = %s", ses.Platform())
	}

	// Second call: should return cached.
	ses2, err := hub.EnsureSession("acc-1", "b1")
	if err != nil {
		t.Fatal(err)
	}
	if ses2 != ses {
		t.Error("expected same session instance")
	}

	if n := hub.SessionCount(); n != 1 {
		t.Errorf("SessionCount = %d, want 1", n)
	}

	// Missing broker.
	ses3, err := hub.EnsureSession("acc-2", "b2")
	if err != nil {
		t.Fatal(err)
	}
	if ses3 != nil {
		t.Error("expected nil session for missing broker")
	}
}

func TestHubCloseSession(t *testing.T) {
	gw := &mockGateway{platform: "mt5", brokerID: "b1"}
	hub := NewHub(func(brokerID string) (Gateway, bool) {
		return gw, true
	}, zap.NewNop())

	hub.EnsureSession("acc-1", "b1")
	if hub.SessionCount() != 1 {
		t.Fatalf("count = %d", hub.SessionCount())
	}
	hub.CloseSession("acc-1")
	if hub.SessionCount() != 0 {
		t.Errorf("count = %d after close", hub.SessionCount())
	}
}

func TestHubActiveSessions(t *testing.T) {
	hub := NewHub(func(brokerID string) (Gateway, bool) {
		return &mockGateway{platform: "mt5", brokerID: brokerID}, true
	}, zap.NewNop())

	hub.EnsureSession("a1", "b1")
	hub.EnsureSession("a2", "b2")
	hub.EnsureSession("a3", "b3")

	active := hub.ActiveSessions()
	if active["mt5"] != 3 {
		t.Errorf("mt5 = %d, want 3", active["mt5"])
	}
}

func TestOrderEventBroker(t *testing.T) {
	b := NewOrderEventBroker()

	ch := b.Subscribe("acc-1")
	if b.SubscriberCount() != 1 {
		t.Errorf("SubscriberCount = %d", b.SubscriberCount())
	}

	ev := &mthubv1.OrderEvent{AccountId: "acc-1", Type: "order_update"}
	b.Publish(ev)

	select {
	case received := <-ch:
		if received.AccountId != "acc-1" {
			t.Errorf("AccountId = %s", received.AccountId)
		}
	default:
		t.Error("expected to receive event")
	}

	b.Unsubscribe("acc-1")
	if b.SubscriberCount() != 0 {
		t.Errorf("SubscriberCount = %d after unsubscribe", b.SubscriberCount())
	}
}

func TestMtHubServiceEnsureSession(t *testing.T) {
	gw := &mockGateway{platform: "mt5", sessionID: "s1", brokerID: "b1"}
	hub := NewHub(func(brokerID string) (Gateway, bool) {
		return gw, true
	}, zap.NewNop())
	svc := NewMtHubService(hub, NewOrderEventBroker(), zap.NewNop())

	req := connect.NewRequest(&mthubv1.EnsureSessionRequest{AccountId: "acc-1"})
	resp, err := svc.EnsureSession(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg.SessionId != "s1" {
		t.Errorf("SessionId = %s", resp.Msg.SessionId)
	}
}

func TestMtHubServiceCloseSession(t *testing.T) {
	gw := &mockGateway{platform: "mt5", brokerID: "b1"}
	hub := NewHub(func(brokerID string) (Gateway, bool) {
		return gw, true
	}, zap.NewNop())
	svc := NewMtHubService(hub, NewOrderEventBroker(), zap.NewNop())

	hub.EnsureSession("acc-1", "b1")
	req := connect.NewRequest(&mthubv1.CloseSessionRequest{AccountId: "acc-1"})
	_, err := svc.CloseSession(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if hub.SessionCount() != 0 {
		t.Error("session not removed")
	}
}

func TestMtHubServiceStubs(t *testing.T) {
	svc := NewMtHubService(
		NewHub(func(string) (Gateway, bool) { return nil, false }, zap.NewNop()),
		NewOrderEventBroker(), zap.NewNop(),
	)

	// OrderSend stub
	r1 := connect.NewRequest(&mthubv1.OrderSendRequest{})
	resp1, err := svc.OrderSend(context.Background(), r1)
	if err != nil {
		t.Fatal(err)
	}
	if resp1.Msg.Error == "" {
		t.Error("expected stub error message")
	}

	// OrderClose stub
	r2 := connect.NewRequest(&mthubv1.OrderCloseRequest{})
	resp2, err := svc.OrderClose(context.Background(), r2)
	if err != nil {
		t.Fatal(err)
	}
	if resp2.Msg.Error == "" {
		t.Error("expected stub error message")
	}

	// SymbolParamsMany stub
	r3 := connect.NewRequest(&mthubv1.SymbolParamsManyRequest{})
	resp3, err := svc.SymbolParamsMany(context.Background(), r3)
	if err != nil {
		t.Fatal(err)
	}
	if resp3.Msg == nil {
		t.Error("expected non-nil response")
	}

	// PriceHistory stub
	r4 := connect.NewRequest(&mthubv1.PriceHistoryRequest{})
	resp4, err := svc.PriceHistory(context.Background(), r4)
	if err != nil {
		t.Fatal(err)
	}
	if resp4.Msg == nil {
		t.Error("expected non-nil response")
	}
}
