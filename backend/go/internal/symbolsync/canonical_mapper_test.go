// Package symbolsync — CanonicalMapper tests.
package symbolsync

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCanonicalizeUpdated(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// Standard cases from original test
		{"eurusd", "EURUSD"},
		{"EURUSD.m", "EURUSD"},
		{"EURUSD.M", "EURUSD"},
		{"EURUSD.ecn", "EURUSD"},
		{"EURUSD.ECN", "EURUSD"},
		{"EURUSD.raw", "EURUSD"},
		{"EURUSD.RAW", "EURUSD"},
		{"EURUSD.pro", "EURUSD"},
		{"EURUSD.PRO", "EURUSD"},
		{"EURUSD.i", "EURUSD"},
		{"EURUSD.I", "EURUSD"},
		{"EURUSD.stp", "EURUSD"},
		{"EURUSD.STP", "EURUSD"},
		// .c is no longer stripped (different contract)
		{"EURUSD.c", "EURUSD.C"},
		{"EURUSD.C", "EURUSD.C"},
		// New: .x suffix
		{"BTCUSD.x", "BTCUSD"},
		{"BTCUSD.X", "BTCUSD"},
		// New: # and ! suffixes
		{"BTCUSD#", "BTCUSD"},
		{"BTCUSD!", "BTCUSD"},
		{"ETHUSD#", "ETHUSD"},
		// Bare M suffix with letter before it
		{"EURUSDm", "EURUSD"},
		{"GBPUSDm", "GBPUSD"},
		// US30m should NOT strip (3 is a digit, prev char before '3' is 'S' which is a letter, but the prev check is on the char before suffix)
		// US30m: suffix 'M', prev char is '0' which is a digit → not stripped
		{"US30m", "US30M"},
		// Short symbol with suffix
		{"US30.PRO", "US30.PRO"}, // len 8 ≤ len(.PRO)+4=8 → no strip
		// Crypto
		{"BTCUSD", "BTCUSD"},
		{"ETHUSD", "ETHUSD"},
	}
	for _, tt := range tests {
		got := Canonicalize(tt.input)
		if got != tt.want {
			t.Errorf("Canonicalize(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCanonicalMapperNoPG(t *testing.T) {
	m := NewCanonicalMapper(nil)
	canonical, partial := m.Resolve(context.Background(), "EURUSDm")
	if canonical != "EURUSD" || partial {
		t.Errorf("EURUSDm → %q partial=%v, want EURUSD false", canonical, partial)
	}

	canonical, partial = m.Resolve(context.Background(), "BTCUSD.x")
	if canonical != "BTCUSD" || partial {
		t.Errorf("BTCUSD.x → %q partial=%v, want BTCUSD false", canonical, partial)
	}
}

func TestCanonicalMapperResolveOrDefault(t *testing.T) {
	m := NewCanonicalMapper(nil)
	if got := m.ResolveOrDefault(context.Background(), "EURUSDm"); got != "EURUSD" {
		t.Errorf("EURUSDm → %q, want EURUSD", got)
	}
	if got := m.ResolveOrDefault(context.Background(), "UNKNOWN123"); got != "UNKNOWN123" {
		t.Errorf("UNKNOWN123 → %q, want UNKNOWN123", got)
	}
}

func TestCanonicalMapperIntegration(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://alfq:WvFId2cgoVQh8eZxTMuYlQyoq6g7Ba4btqnbAuvUCDU@localhost:5432/alfq?sslmode=disable"
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Skipf("PG not available: %v", err)
	}
	defer pool.Close()

	m := NewCanonicalMapper(pool)

	// EURUSDm should resolve to EURUSD via dict
	canonical, partial := m.Resolve(context.Background(), "EURUSDm")
	if partial {
		t.Errorf("EURUSDm should not be partial via dict")
	}
	if canonical != "EURUSD" {
		t.Errorf("EURUSDm → %q, want EURUSD", canonical)
	}

	// BTCUSD should be in dict as crypto
	canonical, partial = m.Resolve(context.Background(), "BTCUSD")
	if partial {
		t.Errorf("BTCUSD should be in dict")
	}
	t.Logf("BTCUSD → %q partial=%v", canonical, partial)

	// Unknown symbol → partial
	_, partial = m.Resolve(context.Background(), "ZZZZZZ999")
	if !partial {
		t.Errorf("ZZZZZZ999 should be partial (not in dict)")
	}

	// Refresh cache
	if err := m.RefreshCache(context.Background()); err != nil {
		t.Errorf("RefreshCache: %v", err)
	}

	// After refresh, cached lookup should work
	canonical, partial = m.Resolve(context.Background(), "EURUSDm")
	if partial {
		t.Errorf("EURUSDm should not be partial after cache refresh")
	}
	if canonical != "EURUSD" {
		t.Errorf("cached EURUSDm → %q, want EURUSD", canonical)
	}
}
