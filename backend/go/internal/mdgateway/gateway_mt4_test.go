package mdgateway

import (
	"testing"
)

func TestNewMT4Gateway(t *testing.T) {
	cfg := AccountConfig{
		Broker:   "test",
		Platform: "mt4",
	}
	g := newMT4Gateway(cfg, nil)
	if g == nil {
		t.Fatal("newMT4Gateway returned nil")
	}
}

func TestMT4Gateway_Platform(t *testing.T) {
	g := newMT4Gateway(AccountConfig{Broker: "broker-1"}, nil)
	if g.Platform() != "mt4" {
		t.Fatalf("expected mt4, got %s", g.Platform())
	}
}

func TestMT4Gateway_BrokerID(t *testing.T) {
	cfg := AccountConfig{Broker: "broker-1"}
	g := newMT4Gateway(cfg, nil)
	if g.BrokerID() != "broker-1" {
		t.Fatalf("expected broker-1, got %s", g.BrokerID())
	}
}

func TestMT4Gateway_BrokerID_2(t *testing.T) {
	cfg := AccountConfig{Broker: "test-broker"}
	g := newMT4Gateway(cfg, nil)
	if g.BrokerID() != "test-broker" {
		t.Fatalf("expected test-broker, got %s", g.BrokerID())
	}
}

func TestMT4Gateway_SessionID(t *testing.T) {
	cfg := AccountConfig{}
	g := newMT4Gateway(cfg, nil)
	session := g.SessionID()
	if session != "" {
		t.Fatalf("expected empty session, got %s", session)
	}
}

func TestMT4Gateway_Conn_NilClient(t *testing.T) {
	cfg := AccountConfig{}
	g := newMT4Gateway(cfg, nil)
	conn := g.Conn()
	if conn != nil {
		t.Fatal("expected nil conn")
	}
}
