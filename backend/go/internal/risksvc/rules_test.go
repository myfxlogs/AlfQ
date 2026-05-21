package risksvc

import (
	"testing"
)

func TestNewSession(t *testing.T) {
	s := NewSession("UTC")
	if s == nil {
		t.Fatal("NewSession returned nil")
	}
}

func TestSession_Name(t *testing.T) {
	s := NewSession("UTC")
	if s.Name() != "session" {
		t.Fatalf("expected session, got %s", s.Name())
	}
}

func TestNewMargin(t *testing.T) {
	m := NewMargin(2.0)
	if m == nil {
		t.Fatal("NewMargin returned nil")
	}
}

func TestMargin_Name(t *testing.T) {
	m := NewMargin(2.0)
	if m.Name() != "margin" {
		t.Fatalf("expected margin, got %s", m.Name())
	}
}
