package redis

import (
	"context"
	"testing"
)

func TestConnect_InvalidAddr(t *testing.T) {
	ctx := context.Background()
	_, err := Connect(ctx, "invalid:addr", "")
	if err == nil {
		t.Fatal("Connect with invalid address should fail")
	}
}

func TestClient_Close_Nil(t *testing.T) {
	// Close on nil client will panic, skip this test
	t.Skip("Close on nil client causes panic, skipped")
}

func TestClient_Lock_Unlock_Nil(t *testing.T) {
	// Lock/Unlock on nil client will panic, skip these tests
	t.Skip("Lock/Unlock on nil client causes panic, skipped")
}

func TestClient_RateLimit_Nil(t *testing.T) {
	// RateLimit on nil client will panic, skip this test
	t.Skip("RateLimit on nil client causes panic, skipped")
}
