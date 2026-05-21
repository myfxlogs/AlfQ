// Package mdgateway — normalizer unit tests.
package mdgateway

import (
	"testing"
)

func TestNormalizerWithResolver(t *testing.T) {
	resolver := NewMapResolver()
	n := NewNormalizer(resolver)

	tick := n.Tick("t1", "b1", "EURUSD.m", 1000, "1.10000", "1.10010")
	if tick.Symbol != "EURUSD.m" {
		t.Errorf("Symbol: got %q, want EURUSD.m", tick.Symbol)
	}
	if tick.Canonical != "EURUSD" {
		t.Errorf("Canonical: got %q, want EURUSD (stripped .m)", tick.Canonical)
	}
}

func TestNormalizerWithoutResolver(t *testing.T) {
	n := NewNormalizer(nil)

	tick := n.Tick("t1", "b1", "EURUSD.m", 1000, "1.10000", "1.10010")
	if tick.Canonical != "EURUSD.m" {
		t.Errorf("Canonical (no resolver): got %q, want EURUSD.m", tick.Canonical)
	}
}

func TestMapResolverCachesResult(t *testing.T) {
	r := NewMapResolver().(*mapResolver)

	c1 := r.Resolve("b1", "EURUSD.m")
	if c1 != "EURUSD" {
		t.Fatalf("first resolve: got %q, want EURUSD", c1)
	}

	// Override cache entry and verify it's returned on second call
	r.Load("b1", "EURUSD.m", "OVERRIDE")
	c2 := r.Resolve("b1", "EURUSD.m")
	if c2 != "OVERRIDE" {
		t.Errorf("cached resolve: got %q, want OVERRIDE", c2)
	}
}

func TestMapResolverFallback(t *testing.T) {
	r := NewMapResolver().(*mapResolver)

	// Symbol with no override: algorithmic fallback
	c := r.Resolve("b1", "GBPJPYm")
	if c != "GBPJPY" {
		t.Errorf("fallback GBPJPYm: got %q, want GBPJPY", c)
	}

	// Same symbol with explicit Load
	r.Load("b1", "GBPJPYm", "GBPJPY-MINI")
	c = r.Resolve("b1", "GBPJPYm")
	if c != "GBPJPY-MINI" {
		t.Errorf("after load: got %q, want GBPJPY-MINI", c)
	}
}
