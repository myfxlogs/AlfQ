package mthub

import (
	"testing"
)

func TestNewClient(t *testing.T) {
	c := NewClient("localhost:9001")
	if c == nil {
		t.Fatal("NewClient returned nil")
	}
}

func TestEnsureSessionResult_Fields(t *testing.T) {
	r := &EnsureSessionResult{
		SessionID:     "test-session",
		AlreadyActive: true,
	}
	if r.SessionID != "test-session" {
		t.Fatalf("expected test-session, got %s", r.SessionID)
	}
	if !r.AlreadyActive {
		t.Fatal("expected AlreadyActive to be true")
	}
}

func TestEnsureSessionResult_Fields2(t *testing.T) {
	r := &EnsureSessionResult{
		SessionID:     "session-2",
		AlreadyActive: false,
	}
	if r.SessionID != "session-2" {
		t.Fatalf("expected session-2, got %s", r.SessionID)
	}
	if r.AlreadyActive {
		t.Fatal("expected AlreadyActive to be false")
	}
}
