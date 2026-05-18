// Package assistantsvc — Cloud LLM provider abstraction.
//
// ADR 0009: ALFQ uses cloud LLM APIs exclusively. No local model deployment.
// This package provides a multi-provider abstraction with failover and cost routing.
package assistantsvc

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Provider represents a cloud LLM API provider.
type Provider struct {
	Name     string            // "openai", "anthropic", "google", "deepseek"
	Endpoint string            // base URL
	Model    string            // default model name
	Headers  map[string]string // auth headers (filled from Vault)
	Timeout  time.Duration
	Priority int // lower = higher priority
}

// ProviderClient abstracts cloud LLM API calls.
type ProviderClient interface {
	Name() string
	Chat(ctx context.Context, systemPrompt, userMessage string) (string, error)
	Embed(ctx context.Context, text string) ([]float32, error)
}

// Router manages multiple cloud LLM providers with failover.
type Router struct {
	mu        sync.RWMutex
	providers []*Provider // sorted by priority
	clients   map[string]ProviderClient
}

// NewRouter creates a provider router.
func NewRouter() *Router {
	return &Router{clients: make(map[string]ProviderClient)}
}

// Register adds a cloud provider to the router (sorted by priority).
func (r *Router) Register(p *Provider, client ProviderClient) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Insert in priority order
	idx := 0
	for idx < len(r.providers) && r.providers[idx].Priority <= p.Priority {
		idx++
	}
	r.providers = append(r.providers, nil)
	copy(r.providers[idx+1:], r.providers[idx:])
	r.providers[idx] = p

	r.clients[p.Name] = client
}

// Chat tries each provider in priority order until one succeeds.
func (r *Router) Chat(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	r.mu.RLock()
	providers := make([]*Provider, len(r.providers))
	copy(providers, r.providers)
	r.mu.RUnlock()

	var lastErr error
	for _, p := range providers {
		client, ok := r.clients[p.Name]
		if !ok {
			continue
		}
		ctx, cancel := context.WithTimeout(ctx, p.Timeout)
		result, err := client.Chat(ctx, systemPrompt, userMessage)
		cancel()
		if err == nil {
			return result, nil
		}
		lastErr = fmt.Errorf("%s: %w", p.Name, err)
	}
	return "", fmt.Errorf("all providers failed: %w", lastErr)
}

// Embed generates embeddings using the highest-priority provider.
func (r *Router) Embed(ctx context.Context, text string) ([]float32, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.providers) == 0 {
		return nil, fmt.Errorf("no providers registered")
	}
	client, ok := r.clients[r.providers[0].Name]
	if !ok {
		return nil, fmt.Errorf("primary provider not found")
	}
	return client.Embed(ctx, text)
}

// List returns registered provider names.
func (r *Router) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, len(r.providers))
	for i, p := range r.providers {
		names[i] = p.Name
	}
	return names
}

// HTTPClient implements ProviderClient for HTTP-based cloud APIs (OpenAI, Anthropic, etc).
type HTTPClient struct {
	name    string
	baseURL string
	model   string
	apiKey  string
}

// NewHTTPClient creates an HTTP-based cloud LLM client.
func NewHTTPClient(name, baseURL, model, apiKey string) *HTTPClient {
	return &HTTPClient{name: name, baseURL: baseURL, model: model, apiKey: apiKey}
}

func (c *HTTPClient) Name() string { return c.name }

func (c *HTTPClient) Chat(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	// TODO: actual HTTP POST to cloud LLM API
	// In production: http.Post(c.baseURL + "/v1/chat/completions", ...)
	return fmt.Sprintf("[%s response to: %s]", c.name, userMessage), nil
}

func (c *HTTPClient) Embed(ctx context.Context, text string) ([]float32, error) {
	// TODO: actual HTTP POST for embeddings
	return make([]float32, 1536), nil
}

// Ensure interface compliance.
var _ ProviderClient = (*HTTPClient)(nil)
