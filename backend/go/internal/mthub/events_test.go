package mthub

import (
	"testing"
)

func TestNewOrderEventBroker(t *testing.T) {
	b := NewOrderEventBroker()
	if b == nil {
		t.Fatal("NewOrderEventBroker returned nil")
	}
}
