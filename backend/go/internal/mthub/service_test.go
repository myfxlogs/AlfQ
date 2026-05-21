package mthub

import (
	"testing"
)

func TestNewMtHubService(t *testing.T) {
	s := NewMtHubService(nil, nil, nil)
	if s == nil {
		t.Fatal("NewMtHubService returned nil")
	}
}

func TestMtHubService_Fields(t *testing.T) {
	s := NewMtHubService(nil, nil, nil)
	if s.hub != nil {
		t.Fatal("expected hub to be nil")
	}
	if s.events != nil {
		t.Fatal("expected events to be nil")
	}
	if s.log != nil {
		t.Fatal("expected log to be nil")
	}
}
