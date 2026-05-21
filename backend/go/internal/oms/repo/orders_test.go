package repo

import (
	"testing"
)

func TestNewOrderRepo(t *testing.T) {
	r := NewOrderRepo(nil)
	if r == nil {
		t.Fatal("NewOrderRepo returned nil")
	}
}

func TestOrderRepo_Fields(t *testing.T) {
	r := NewOrderRepo(nil)
	if r.pool != nil {
		t.Fatal("expected pool to be nil")
	}
}
