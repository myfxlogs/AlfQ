package mt4

import (
	"testing"
)

func TestDefaultEndpoint(t *testing.T) {
	if DefaultEndpoint == "" {
		t.Fatal("DefaultEndpoint should not be empty")
	}
	if DefaultEndpoint != "mt4grpc3.mtapi.io:443" {
		t.Fatalf("expected mt4grpc3.mtapi.io:443, got %s", DefaultEndpoint)
	}
}

func TestClient_Conn(t *testing.T) {
	c := &Client{cc: nil}
	conn := c.Conn()
	if conn != nil {
		t.Fatal("expected nil conn")
	}
}

func TestClient_Fields(t *testing.T) {
	c := &Client{cc: nil}
	if c.cc != nil {
		t.Fatal("expected cc to be nil")
	}
}
