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
