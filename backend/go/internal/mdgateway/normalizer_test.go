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

func TestNewMapResolver(t *testing.T) {
	r := NewMapResolver()
	if r == nil {
		t.Fatal("NewMapResolver returned nil")
	}
}

func TestNormalizer_TickFields(t *testing.T) {
	n := NewNormalizer(nil)
	tick := n.Tick("tenant1", "broker1", "EURUSD", 1234567890, "1.1000", "1.1005")
	
	if tick.TenantId != "tenant1" {
		t.Fatalf("TenantId: got %s, want tenant1", tick.TenantId)
	}
	if tick.Broker != "broker1" {
		t.Fatalf("Broker: got %s, want broker1", tick.Broker)
	}
	if tick.Symbol != "EURUSD" {
		t.Fatalf("Symbol: got %s, want EURUSD", tick.Symbol)
	}
	if tick.TsUnixMs != 1234567890 {
		t.Fatalf("TsUnixMs: got %d, want 1234567890", tick.TsUnixMs)
	}
	if tick.Bid.GetValue() != "1.1000" {
		t.Fatalf("Bid: got %s, want 1.1000", tick.Bid.GetValue())
	}
	if tick.Ask.GetValue() != "1.1005" {
		t.Fatalf("Ask: got %s, want 1.1005", tick.Ask.GetValue())
	}
}
