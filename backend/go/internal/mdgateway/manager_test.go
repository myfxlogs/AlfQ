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
