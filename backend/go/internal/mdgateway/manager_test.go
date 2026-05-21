package mdgateway

import (
	"testing"
)

func TestNewEmptyManager(t *testing.T) {
	m := NewEmptyManager()
	if m == nil {
		t.Fatal("NewEmptyManager returned nil")
	}
	if m.gateways == nil {
		t.Fatal("gateways map not initialized")
	}
	if len(m.gateways) != 0 {
		t.Fatalf("expected 0 gateways, got %d", len(m.gateways))
	}
}

func TestManager_Connections(t *testing.T) {
	m := NewEmptyManager()
	conns := m.Connections()
	if conns == nil {
		t.Fatal("Connections returned nil")
	}
	if len(conns) != 0 {
		t.Fatalf("expected 0 connections, got %d", len(conns))
	}
}

func TestManager_RemoveGateway(t *testing.T) {
	m := NewEmptyManager()
	// Remove non-existent gateway should not panic
	m.RemoveGateway("test-key")
}

func TestAccountConfig_Fields(t *testing.T) {
	cfg := AccountConfig{
		Broker:     "test-broker",
		Platform:   "mt5",
		Login:      "12345",
		Password:   "password",
		Server:     "server:443",
		Host:       "server",
		Port:       "443",
		MtapiToken: "token",
		TenantID:   "tenant-1",
	}
	if cfg.Broker != "test-broker" {
		t.Fatalf("expected test-broker, got %s", cfg.Broker)
	}
	if cfg.Platform != "mt5" {
		t.Fatalf("expected mt5, got %s", cfg.Platform)
	}
}

func TestAccountEntry_Fields(t *testing.T) {
	entry := AccountEntry{
		TenantID: "tenant-1",
		Broker:   "test-broker",
		Platform: "mt5",
		Login:    "12345",
		Password: "password",
		Server:   "server:443",
		Host:     "server",
		Port:     "443",
		Symbols:  []string{"EURUSD", "GBPJPY"},
	}
	if entry.TenantID != "tenant-1" {
		t.Fatalf("expected tenant-1, got %s", entry.TenantID)
	}
	if len(entry.Symbols) != 2 {
		t.Fatalf("expected 2 symbols, got %d", len(entry.Symbols))
	}
}

func TestConfig_Fields(t *testing.T) {
	cfg := Config{
		Accounts: []AccountEntry{},
		Log:      LogConfig{Level: "info"},
	}
	if cfg.Log.Level != "info" {
		t.Fatalf("expected info, got %s", cfg.Log.Level)
	}
}
