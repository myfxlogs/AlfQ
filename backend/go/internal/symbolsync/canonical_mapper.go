// Package symbolsync — canonical mapper: dict-first + rule-fallback.
// Refs: docs/design/multi-broker-symbol.md §5.4
package symbolsync

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CanonicalMapper resolves symbol_raw → canonical using the canonical_symbols dictionary
// with rule-based fallback for known suffix patterns.
type CanonicalMapper struct {
	pool  *pgxpool.Pool
	mu    sync.RWMutex
	cache map[string]string // symbol_raw_upper → canonical, refreshed periodically
}

// NewCanonicalMapper creates a canonical mapper backed by the canonical_symbols PG dict.
func NewCanonicalMapper(pool *pgxpool.Pool) *CanonicalMapper {
	return &CanonicalMapper{pool: pool, cache: make(map[string]string)}
}

// Resolve resolves a symbol_raw to canonical.
// Strategy: cache lookup → dict exact match on symbol_raw_upper → rule-based Canonicalize.
func (m *CanonicalMapper) Resolve(ctx context.Context, symbolRaw string) (canonical string, partial bool) {
	upper := strings.ToUpper(symbolRaw)

	// 1. Cache hit
	m.mu.RLock()
	if c, ok := m.cache[upper]; ok {
		m.mu.RUnlock()
		return c, c == ""
	}
	m.mu.RUnlock()

	// 2. Rule-based normalization as fast path
	canonical = Canonicalize(upper)

	// 3. Validate against dictionary if PG is available
	if m.pool != nil {
		var exists bool
		err := m.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM canonical_symbols WHERE canonical = $1 AND enabled = true)`,
			canonical,
		).Scan(&exists)
		if err == nil && exists {
			m.cacheSet(upper, canonical)
			return canonical, false
		}
		// Not in dict: try direct match of symbol_raw_upper in dict
		err = m.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM canonical_symbols WHERE canonical = $1 AND enabled = true)`,
			upper,
		).Scan(&exists)
		if err == nil && exists {
			m.cacheSet(upper, upper)
			return upper, false
		}
		// Neither found → partial
		m.cacheSet(upper, "")
		return canonical, true
	}

	// No PG: return best-effort canonical
	return canonical, false
}

// ResolveOrDefault is like Resolve but returns the raw as-is when dict lookup fails.
func (m *CanonicalMapper) ResolveOrDefault(ctx context.Context, symbolRaw string) string {
	canonical, partial := m.Resolve(ctx, symbolRaw)
	if partial {
		return strings.ToUpper(symbolRaw) // best-effort uppercase
	}
	return canonical
}

// RefreshCache reloads the symbol_raw → canonical mapping from broker_symbols.
func (m *CanonicalMapper) RefreshCache(ctx context.Context) error {
	if m.pool == nil {
		return fmt.Errorf("canonical mapper: pg not available")
	}
	rows, err := m.pool.Query(ctx,
		`SELECT symbol_raw, canonical FROM broker_symbols WHERE canonical IS NOT NULL AND canonical != ''`)
	if err != nil {
		return fmt.Errorf("canonical mapper: refresh: %w", err)
	}
	defer rows.Close()

	newCache := make(map[string]string)
	for rows.Next() {
		var raw, canon string
		if err := rows.Scan(&raw, &canon); err != nil {
			continue
		}
		newCache[strings.ToUpper(raw)] = canon
	}

	m.mu.Lock()
	m.cache = newCache
	m.mu.Unlock()
	return nil
}

func (m *CanonicalMapper) cacheSet(key, value string) {
	m.mu.Lock()
	m.cache[key] = value
	m.mu.Unlock()
}
