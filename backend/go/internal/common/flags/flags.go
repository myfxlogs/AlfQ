// Package flags provides a feature flag client per docs/17.
// Flags are evaluated based on context (tenant_id, env) and rollout rules.
package flags

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"sync"
	"time"
)

// FlagType represents the value type of a feature flag.
type FlagType string

const (
	FlagTypeBool   FlagType = "bool"
	FlagTypeInt    FlagType = "int"
	FlagTypeString FlagType = "string"
)

// FlagDefinition defines a feature flag with rollout rules.
type FlagDefinition struct {
	Key         string      `json:"key"`
	Description string      `json:"description"`
	Type        FlagType    `json:"type"`
	DefaultVal  any         `json:"default_val"`
	Rollout     Rollout     `json:"rollout"`
	Owner       string      `json:"owner"`
	ExpiresAt   *time.Time  `json:"expires_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// Rollout defines when a flag is enabled.
type Rollout struct {
	Rules []RolloutRule `json:"rules"`
}

// RolloutRule is a single rule for evaluating a flag.
type RolloutRule struct {
	If      *RolloutCondition `json:"if,omitempty"`  // must match all conditions
	Percent float64           `json:"percent,omitempty"` // percentage-based
	Salt    string            `json:"salt,omitempty"`
	Value   any               `json:"value"`          // value when rule matches
}

// RolloutCondition specifies conditions for rule matching.
type RolloutCondition struct {
	TenantID []string `json:"tenant_id,omitempty"`
	Env      []string `json:"env,omitempty"`
}

// FlagContext provides evaluation context.
type FlagContext struct {
	TenantID string
	Env      string
}

// Client evaluates feature flags.
type Client struct {
	mu    sync.RWMutex
	flags map[string]*FlagDefinition
}

// NewClient creates a feature flag client.
func NewClient() *Client {
	return &Client{flags: make(map[string]*FlagDefinition)}
}

// Register adds a flag definition.
func (c *Client) Register(def FlagDefinition) {
	c.mu.Lock()
	c.flags[def.Key] = &def
	c.mu.Unlock()
}

// Bool evaluates a boolean flag. Returns defaultVal if flag not found.
func (c *Client) Bool(ctx context.Context, key string, defaultVal bool) bool {
	return c.eval(key, defaultVal, func(v any) bool {
		b, ok := v.(bool)
		return ok && b
	})
}

// Int evaluates an integer flag.
func (c *Client) Int(ctx context.Context, key string, defaultVal int) int {
	return c.eval(key, defaultVal, func(v any) int {
		switch x := v.(type) {
		case float64:
			return int(x)
		case int:
			return x
		}
		return defaultVal
	})
}

// String evaluates a string flag.
func (c *Client) String(ctx context.Context, key string, defaultVal string) string {
	return c.eval(key, defaultVal, func(v any) string {
		s, ok := v.(string)
		if ok {
			return s
		}
		return defaultVal
	})
}

// eval runs the rollout rules against context and returns the matched value.
func (c *Client) eval[T any](key string, defaultVal T, convert func(any) T) T {
	c.mu.RLock()
	def, ok := c.flags[key]
	c.mu.RUnlock()
	if !ok {
		return defaultVal
	}

	// Check expiration
	if def.ExpiresAt != nil && time.Now().After(*def.ExpiresAt) {
		v, ok := def.DefaultVal.(T)
		if ok {
			return v
		}
		return defaultVal
	}

	// Build context (from request context in production)
	ctx := FlagContext{Env: "production"}

	// Evaluate rules in order
	for _, rule := range def.Rollout.Rules {
		if c.matchRule(&rule, &def.Key, &ctx) {
			return convert(rule.Value)
		}
	}

	v, ok := def.DefaultVal.(T)
	if ok {
		return v
	}
	return defaultVal
}

// matchRule checks if a rollout rule matches the current context.
func (c *Client) matchRule(rule *RolloutRule, key *string, ctx *FlagContext) bool {
	// Condition matching
	if rule.If != nil {
		cond := rule.If
		if len(cond.TenantID) > 0 {
			found := false
			for _, t := range cond.TenantID {
				if ctx.TenantID == t {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		if len(cond.Env) > 0 {
			found := false
			for _, e := range cond.Env {
				if ctx.Env == e {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	// Percentage-based rollout
	if rule.Percent > 0 && key != nil {
		salt := rule.Salt
		if salt == "" {
			salt = *key
		}
		h := sha256.Sum256([]byte(salt + ctx.TenantID + ctx.Env))
		bucket := binary.BigEndian.Uint64(h[:8]) % 100
		return float64(bucket) < rule.Percent
	}

	return true
}

// Reload replaces the current flag set (for hot-reload).
func (c *Client) Reload(defs []FlagDefinition) {
	c.mu.Lock()
	c.flags = make(map[string]*FlagDefinition, len(defs))
	for i := range defs {
		c.flags[defs[i].Key] = &defs[i]
	}
	c.mu.Unlock()
}

// List returns all registered flags.
func (c *Client) List() []FlagDefinition {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]FlagDefinition, 0, len(c.flags))
	for _, f := range c.flags {
		out = append(out, *f)
	}
	return out
}

// Ensure imports are used
var _ = json.Marshal
