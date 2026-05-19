// Package vault provides HashiCorp Vault client utilities.
package vault

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	vault "github.com/hashicorp/vault/api"
)

// Client wraps Vault API access via the official hashicorp/vault/api.
type Client struct {
	vc *vault.Client
}

// New creates a Vault client from VAULT_ADDR / VAULT_TOKEN environment variables.
func New() (*Client, error) {
	cfg := vault.DefaultConfig()
	if cfg.Error != nil {
		return nil, fmt.Errorf("vault: default config: %w", cfg.Error)
	}
	vc, err := vault.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("vault: new client: %w", err)
	}
	if vc.Token() == "" {
		return nil, fmt.Errorf("vault: VAULT_TOKEN not set")
	}
	return &Client{vc: vc}, nil
}

// ReadSecret reads a secret from the KV v2 engine at the given path.
// The path is relative to the secret mount (default "secret").
func (c *Client) ReadSecret(ctx context.Context, path string) (map[string]any, error) {
	s, err := c.vc.KVv2("secret").Get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("vault: read secret %s: %w", path, err)
	}
	return s.Data, nil
}

// Encrypt encrypts plaintext using Vault Transit engine.
// key is the transit key name (e.g. "alfq-account-password").
func (c *Client) Encrypt(ctx context.Context, key, plaintext string) (string, error) {
	payload := map[string]any{
		"plaintext": base64.StdEncoding.EncodeToString([]byte(plaintext)),
	}
	s, err := c.vc.Logical().Write("transit/encrypt/"+key, payload)
	if err != nil {
		return "", fmt.Errorf("vault: transit encrypt %s: %w", key, err)
	}
	if s == nil || s.Data == nil {
		return "", fmt.Errorf("vault: transit encrypt %s: empty response", key)
	}
	ciphertext, ok := s.Data["ciphertext"].(string)
	if !ok {
		return "", fmt.Errorf("vault: transit encrypt %s: ciphertext field missing", key)
	}
	return ciphertext, nil
}

// Decrypt decrypts ciphertext using Vault Transit engine.
// key is the transit key name.
func (c *Client) Decrypt(ctx context.Context, key, ciphertext string) (string, error) {
	payload := map[string]any{
		"ciphertext": ciphertext,
	}
	s, err := c.vc.Logical().Write("transit/decrypt/"+key, payload)
	if err != nil {
		return "", fmt.Errorf("vault: transit decrypt %s: %w", key, err)
	}
	if s == nil || s.Data == nil {
		return "", fmt.Errorf("vault: transit decrypt %s: empty response", key)
	}
	encoded, ok := s.Data["plaintext"].(string)
	if !ok {
		return "", fmt.Errorf("vault: transit decrypt %s: plaintext field missing", key)
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("vault: transit decrypt %s: decode: %w", key, err)
	}
	return string(decoded), nil
}

// ensure unused imports don't trigger compile errors
var _ = os.Getenv
