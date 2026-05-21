package repo

import (
	"testing"
)

func TestNewPositionRepo(t *testing.T) {
	r := NewPositionRepo(nil)
	if r == nil {
		t.Fatal("NewPositionRepo returned nil")
	}
}

func TestPositionRepo_Fields(t *testing.T) {
	r := NewPositionRepo(nil)
	if r.pool != nil {
		t.Fatal("expected pool to be nil")
	}
}
