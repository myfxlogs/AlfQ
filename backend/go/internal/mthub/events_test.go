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

func TestOrderEventBroker_SubscriberCount(t *testing.T) {
	b := NewOrderEventBroker()
	if b.SubscriberCount() != 0 {
		t.Fatalf("expected 0, got %d", b.SubscriberCount())
	}
}

func TestOrderEventBroker_Unsubscribe(t *testing.T) {
	b := NewOrderEventBroker()
	b.Unsubscribe("test-account") // Should not panic
}
