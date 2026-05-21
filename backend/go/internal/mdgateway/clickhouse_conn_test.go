package mdgateway

import (
	"testing"
)

func TestDefaultCHConnConfig(t *testing.T) {
	cfg := DefaultCHConnConfig()
	if cfg.Addr != "localhost:9000" {
		t.Fatalf("expected localhost:9000, got %s", cfg.Addr)
	}
	if cfg.Database != "alfq" {
		t.Fatalf("expected alfq, got %s", cfg.Database)
	}
	if cfg.User != "alfq" {
		t.Fatalf("expected alfq, got %s", cfg.User)
	}
	if cfg.Password != "" {
		t.Fatalf("expected empty password, got %s", cfg.Password)
	}
}

func TestCHConnConfig_Fields(t *testing.T) {
	cfg := CHConnConfig{
		Addr:     "example.com:9000",
		Database: "mydb",
		User:     "myuser",
		Password: "mypass",
	}
	if cfg.Addr != "example.com:9000" {
		t.Fatalf("expected example.com:9000, got %s", cfg.Addr)
	}
	if cfg.Database != "mydb" {
		t.Fatalf("expected mydb, got %s", cfg.Database)
	}
}

func TestCHConn_Close_Nil(t *testing.T) {
	c := &CHConn{}
	err := c.Close()
	if err != nil {
		t.Fatalf("Close on nil conn should return nil, got %v", err)
	}
}
