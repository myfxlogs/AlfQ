// Package vault provides a minimal HashiCorp Vault client for reading secrets.
// Uses the Vault HTTP API directly (no SDK dependency) to keep the binary small.
package vault

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// Client reads secrets from a Vault KV-v2 engine.
type Client struct {
	addr  string
	token string
	hc    *http.Client
}

// New creates a Vault client from environment variables.
// VAULT_ADDR and VAULT_TOKEN must be set.
func New() (*Client, error) {
	addr := os.Getenv("VAULT_ADDR")
	if addr == "" {
		addr = "http://vault:8200"
	}
	token := os.Getenv("VAULT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("vault: VAULT_TOKEN not set")
	}
	return &Client{addr: addr, token: token, hc: &http.Client{}}, nil
}

// LoadSecrets reads all secrets under the given KV-v2 path.
// Returns map of key → value.
func (c *Client) LoadSecrets(ctx context.Context, path string) (map[string]string, error) {
	url := fmt.Sprintf("%s/v1/%s/data", c.addr, path)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("vault: request: %w", err)
	}
	req.Header.Set("X-Vault-Token", c.token)

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vault: get %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("vault: %s: %s", resp.Status, string(body))
	}

	var result struct {
		Data struct {
			Data map[string]string `json:"data"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("vault: decode: %w", err)
	}
	if result.Data.Data == nil {
		return nil, fmt.Errorf("vault: no data at %s", path)
	}

	// Normalize keys: Vault returns lowercase, convert env-style keys back.
	out := make(map[string]string, len(result.Data.Data))
	for k, v := range result.Data.Data {
		out[strings.ToUpper(k)] = v
	}
	return out, nil
}
