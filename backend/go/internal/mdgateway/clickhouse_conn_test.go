package mdgateway

import (
	"testing"
)

func TestDefaultCHConnConfig(t *testing.T) {
	cfg := DefaultCHConnConfig()
	if cfg.Addr != "localhost:9000" {
		t.Fatalf("expected Addr=localhost:9000, got %s", cfg.Addr)
	}
	if cfg.Database != "alfq" {
		t.Fatalf("expected Database=alfq, got %s", cfg.Database)
	}
	if cfg.User != "alfq" {
		t.Fatalf("expected User=alfq, got %s", cfg.User)
	}
	if cfg.Password != "" {
		t.Fatalf("expected Password='', got %s", cfg.Password)
	}
}

func TestCHConn_Close_Nil(t *testing.T) {
	c := &CHConn{}
	err := c.Close()
	if err != nil {
		t.Fatalf("Close on nil conn should return nil, got %v", err)
	}
}
