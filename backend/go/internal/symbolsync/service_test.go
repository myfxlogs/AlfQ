package symbolsync

import (
	"testing"
)

func TestNewService(t *testing.T) {
	s := NewService(nil, nil)
	if s == nil {
		t.Fatal("NewService returned nil")
	}
}

func TestSyncParams_Fields(t *testing.T) {
	params := SyncParams{
		BrokerID:  "broker-1",
		Platform:  "MT5",
		SessionID: "session-123",
		Conn:      nil,
	}
	if params.BrokerID != "broker-1" {
		t.Fatalf("expected broker-1, got %s", params.BrokerID)
	}
	if params.Platform != "MT5" {
		t.Fatalf("expected MT5, got %s", params.Platform)
	}
}
