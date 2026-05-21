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
