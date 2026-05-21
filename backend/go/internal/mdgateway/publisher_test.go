package mdgateway

import (
	"testing"
)

func TestNewPublisher(t *testing.T) {
	p := NewPublisher(nil, "nats://localhost:4222")
	if p == nil {
		t.Fatal("NewPublisher returned nil")
	}
	if p.natsURL != "nats://localhost:4222" {
		t.Fatalf("expected nats://localhost:4222, got %s", p.natsURL)
	}
}

func TestPublisher_Fields(t *testing.T) {
	p := NewPublisher(nil, "nats://localhost:4222")
	if p.nc != nil {
		t.Fatal("expected nc to be nil")
	}
	if p.js != nil {
		t.Fatal("expected js to be nil")
	}
}

func TestPublisher_Close_NilConn(t *testing.T) {
	p := NewPublisher(nil, "nats://localhost:4222")
	err := p.Close()
	if err != nil {
		t.Fatalf("Close on nil conn should return nil, got %v", err)
	}
}

func TestPublisher_PublishRaw_NilConn(t *testing.T) {
	p := NewPublisher(nil, "nats://localhost:4222")
	err := p.PublishRaw("test.subject", []byte("test"))
	if err != nil {
		t.Fatalf("PublishRaw with nil conn should return nil, got %v", err)
	}
}

func TestPublisher_NewPublisher_DifferentURL(t *testing.T) {
	p := NewPublisher(nil, "nats://example.com:4222")
	if p.natsURL != "nats://example.com:4222" {
		t.Fatalf("expected nats://example.com:4222, got %s", p.natsURL)
	}
}
