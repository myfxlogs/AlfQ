package mthub

import (
	"testing"
)

func TestNewHub(t *testing.T) {
	h := NewHub(nil, nil)
	if h == nil {
		t.Fatal("NewHub returned nil")
	}
	if h.sessions == nil {
		t.Fatal("sessions map not initialized")
	}
}
