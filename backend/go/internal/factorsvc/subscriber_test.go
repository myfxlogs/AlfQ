package factorsvc

import (
	"testing"
)

func TestNewSubscriber(t *testing.T) {
	s := NewSubscriber(nil, "nats://localhost:4222", nil)
	if s == nil {
		t.Fatal("NewSubscriber returned nil")
	}
	if s.natsURL != "nats://localhost:4222" {
		t.Fatalf("expected nats://localhost:4222, got %s", s.natsURL)
	}
}
