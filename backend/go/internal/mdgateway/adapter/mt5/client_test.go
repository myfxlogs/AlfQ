package mt5

import (
	"testing"
)

func TestDefaultEndpoint(t *testing.T) {
	if DefaultEndpoint == "" {
		t.Fatal("DefaultEndpoint should not be empty")
	}
}

func TestClient_Conn(t *testing.T) {
	c := &Client{cc: nil}
	conn := c.Conn()
	if conn != nil {
		t.Fatal("expected nil conn")
	}
}
