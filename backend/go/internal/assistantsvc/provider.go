// Package assistantsvc — Cloud LLM provider abstraction.
//
// ADR 0009: ALFQ uses cloud LLM APIs exclusively. No local model deployment.
// This package provides a multi-provider abstraction with failover and cost routing.
//
// R10: Chat now returns structured result with usage info for cost tracking.
package assistantsvc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	Chat(ctx context.Context, systemPrompt, userMessage string) (*ChatResult, error)
	Embed(ctx context.Context, text string) ([]float32, error)
}

// ChatResult holds the chat response and usage metadata (R10).
type ChatResult struct {
	Content   string `json:"content"`
	TokensIn  int    `json:"tokens_in"`
	TokensOut int    `json:"tokens_out"`
	Model     string `json:"model"`
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
func (r *Router) Chat(ctx context.Context, systemPrompt, userMessage string) (*ChatResult, error) {
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
	return nil, fmt.Errorf("all providers failed: %w", lastErr)
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

func (c *HTTPClient) Chat(ctx context.Context, systemPrompt, userMessage string) (*ChatResult, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("assistant: %s api key not configured", c.name)
	}
	body := map[string]any{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userMessage},
		},
	}
	return c.doChatRequest(ctx, c.baseURL+"/v1/chat/completions", body)
}

func (c *HTTPClient) Embed(ctx context.Context, text string) ([]float32, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("assistant: %s api key not configured", c.name)
	}
	body := map[string]any{
		"model": "text-embedding-3-small",
		"input": text,
	}
	resp, err := c.doJSONRequest(ctx, c.baseURL+"/v1/embeddings", body)
	if err != nil {
		return nil, err
	}
	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &result); err != nil || len(result.Data) == 0 {
		return nil, fmt.Errorf("assistant: embed: no embedding returned")
	}
	return result.Data[0].Embedding, nil
}

func (c *HTTPClient) doChatRequest(ctx context.Context, url string, body any) (*ChatResult, error) {
	respBody, err := c.doJSONRequest(ctx, url, body)
	if err != nil {
		return nil, err
	}
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
		Model string `json:"model"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("assistant: chat response: %w", err)
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("assistant: no choices returned")
	}
	return &ChatResult{
		Content:   result.Choices[0].Message.Content,
		TokensIn:  result.Usage.PromptTokens,
		TokensOut: result.Usage.CompletionTokens,
		Model:     result.Model,
	}, nil
}

func (c *HTTPClient) doJSONRequest(ctx context.Context, url string, body any) ([]byte, error) {
	payload, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("assistant: request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("assistant: %s: %w", c.name, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("assistant: %s: %s (%s)", c.name, resp.Status, string(respBody[:min(len(respBody), 200)]))
	}
	return respBody, nil
}

// Ensure interface compliance.
var _ ProviderClient = (*HTTPClient)(nil)
