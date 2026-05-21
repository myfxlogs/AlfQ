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
