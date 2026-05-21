package ssehub

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewHub(t *testing.T) {
	h := New()
	if h == nil {
		t.Fatal("New() returned nil")
	}
	if h.clients == nil {
		t.Fatal("clients map not initialized")
	}
	if h.ClientCount() != 0 {
		t.Fatalf("expected 0 clients, got %d", h.ClientCount())
	}
}

func TestHubBroadcast(t *testing.T) {
	h := New()
	// Broadcast with no clients should not panic
	h.Broadcast([]byte("test"))

	// Add a mock client
	ch := make(chan []byte, 64)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()

	h.Broadcast([]byte("hello"))
	select {
	case msg := <-ch:
		if string(msg) != "hello" {
			t.Fatalf("expected hello, got %s", string(msg))
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for broadcast")
	}
}

func TestHubClientCount(t *testing.T) {
	h := New()
	if h.ClientCount() != 0 {
		t.Fatalf("expected 0, got %d", h.ClientCount())
	}

	ch := make(chan []byte, 64)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()

	if h.ClientCount() != 1 {
		t.Fatalf("expected 1, got %d", h.ClientCount())
	}
}

func TestHubServeHTTP(t *testing.T) {
	h := New()
	w := httptest.NewRecorder()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	r := httptest.NewRequest("GET", "/events", nil).WithContext(ctx)

	h.ServeHTTP(w, r)

	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("expected text/event-stream, got %s", w.Header().Get("Content-Type"))
	}
}

