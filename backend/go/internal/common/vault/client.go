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

// LoadSecrets reads all secrets under the given KV-v2 mount path.
// Returns map of key → value.
func (c *Client) LoadSecrets(ctx context.Context, path string) (map[string]string, error) {
	// First, list all keys under the mount.
	keys, err := c.listKeys(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("vault: list keys: %w", err)
	}

	out := make(map[string]string, len(keys))
	for _, k := range keys {
		val, err := c.readKey(ctx, path, k)
		if err != nil {
			// Skip keys we can't read; they may be sub-paths or different types.
			continue
		}
		out[strings.ToUpper(k)] = val
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("vault: no readable keys at %s", path)
	}
	return out, nil
}

// listKeys returns all top-level keys under a KV-v2 mount.
func (c *Client) listKeys(ctx context.Context, path string) ([]string, error) {
	url := fmt.Sprintf("%s/v1/%s/metadata?list=true", c.addr, path)
	req, err := http.NewRequestWithContext(ctx, "LIST", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Vault-Token", c.token)

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("vault: list %s: %s: %s", url, resp.Status, string(body))
	}

	var result struct {
		Data struct {
			Keys []string `json:"keys"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Data.Keys, nil
}

// readKey reads a single secret from a KV-v2 mount.
func (c *Client) readKey(ctx context.Context, mount, key string) (string, error) {
	url := fmt.Sprintf("%s/v1/%s/data/%s", c.addr, mount, key)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-Vault-Token", c.token)

	resp, err := c.hc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("vault: read %s: %s", url, resp.Status)
	}

	var result struct {
		Data struct {
			Data map[string]string `json:"data"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if v, ok := result.Data.Data["value"]; ok {
		return v, nil
	}
	return "", fmt.Errorf("vault: no 'value' field at %s", key)
}
