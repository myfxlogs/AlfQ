package mthub

import (
	"testing"
)

func TestNewHub(t *testing.T) {
	h := NewHub(nil, nil)
	if h == nil {
		t.Fatal("NewHub returned nil")
	}
}

func TestHub_Fields(t *testing.T) {
	h := NewHub(nil, nil)
	if h.lookupGW != nil {
		t.Fatal("expected lookupGW to be nil")
	}
	if h.log != nil {
		t.Fatal("expected log to be nil")
	}
	if h.sessions == nil {
		t.Fatal("sessions map not initialized")
	}
}
