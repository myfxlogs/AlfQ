package mdgateway

import (
	"testing"
)

func TestNewMT5Gateway(t *testing.T) {
	cfg := AccountConfig{
		Broker:   "test",
		Platform: "mt5",
	}
	g := newMT5Gateway(cfg, nil)
	if g == nil {
		t.Fatal("newMT5Gateway returned nil")
	}
}

func TestMT5Gateway_Platform(t *testing.T) {
	cfg := AccountConfig{Platform: "mt5"}
	g := newMT5Gateway(cfg, nil)
	if g.Platform() != "mt5" {
		t.Fatalf("expected mt5, got %s", g.Platform())
	}
}

func TestMT5Gateway_BrokerID(t *testing.T) {
	cfg := AccountConfig{Broker: "test-broker"}
	g := newMT5Gateway(cfg, nil)
	if g.BrokerID() != "test-broker" {
		t.Fatalf("expected test-broker, got %s", g.BrokerID())
	}
}

func TestMT5Gateway_SessionID(t *testing.T) {
	cfg := AccountConfig{}
	g := newMT5Gateway(cfg, nil)
	session := g.SessionID()
	if session != "" {
		t.Fatalf("expected empty session, got %s", session)
	}
}

func TestMT5Gateway_Conn_NilClient(t *testing.T) {
	cfg := AccountConfig{}
	g := newMT5Gateway(cfg, nil)
	conn := g.Conn()
	if conn != nil {
		t.Fatal("expected nil conn")
	}
}
