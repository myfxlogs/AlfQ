package assistantsvc

import (
	"context"
	"testing"
	"time"
)

type mockClient struct {
	name    string
	chatErr error
}

func (m *mockClient) Name() string                                     { return m.name }
func (m *mockClient) Chat(ctx context.Context, sys, msg string) (string, error) {
	if m.chatErr != nil {
		return "", m.chatErr
	}
	return "mock response", nil
}
func (m *mockClient) Embed(ctx context.Context, text string) ([]float32, error) {
	return make([]float32, 1536), nil
}

func TestRouterRegisterAndChat(t *testing.T) {
	router := NewRouter()
	router.Register(
		&Provider{Name: "mock", Endpoint: "http://mock", Model: "mock-v1", Priority: 1, Timeout: 5 * time.Second},
		&mockClient{name: "mock"},
	)

	result, err := router.Chat(context.Background(), "system", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if result != "mock response" {
		t.Fatalf("expected 'mock response', got %q", result)
	}
}

func TestRouterList(t *testing.T) {
	router := NewRouter()
	router.Register(&Provider{Name: "p1", Priority: 1, Timeout: time.Second}, &mockClient{name: "p1"})
	router.Register(&Provider{Name: "p2", Priority: 2, Timeout: time.Second}, &mockClient{name: "p2"})

	names := router.List()
	if len(names) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(names))
	}
	if names[0] != "p1" || names[1] != "p2" {
		t.Fatalf("expected [p1 p2], got %v", names)
	}
}

func TestRouterEmbed(t *testing.T) {
	router := NewRouter()
	router.Register(&Provider{Name: "mock", Priority: 1, Timeout: time.Second}, &mockClient{name: "mock"})

	vec, err := router.Embed(context.Background(), "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(vec) != 1536 {
		t.Fatalf("expected 1536 dims, got %d", len(vec))
	}
}

func TestRouterEmbedNoProviders(t *testing.T) {
	router := NewRouter()
	_, err := router.Embed(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error with no providers")
	}
}

func TestHTTPClientName(t *testing.T) {
	c := NewHTTPClient("test", "http://x", "m1", "key")
	if c.Name() != "test" {
		t.Fatalf("expected 'test', got %s", c.Name())
	}
}

func TestHTTPClientChat(t *testing.T) {
	c := NewHTTPClient("test", "http://x", "m1", "key")
	resp, err := c.Chat(context.Background(), "sys", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if resp == "" {
		t.Fatal("expected non-empty response")
	}
}

func TestHTTPClientEmbed(t *testing.T) {
	c := NewHTTPClient("test", "http://x", "m1", "key")
	vec, err := c.Embed(context.Background(), "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(vec) != 1536 {
		t.Fatalf("expected 1536 dims, got %d", len(vec))
	}
}
