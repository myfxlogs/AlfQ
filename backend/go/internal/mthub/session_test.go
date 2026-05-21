package mthub

import (
	"testing"
)

func TestSession_Fields(t *testing.T) {
	// Just test that Session struct can be created
	s := &Session{
		AccountID: "test-account",
		Gateway:   nil,
	}
	if s.AccountID != "test-account" {
		t.Fatalf("expected test-account, got %s", s.AccountID)
	}
}
