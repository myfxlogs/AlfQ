// Package symbolsync — canonical symbol name tests.
package symbolsync

import (
	"testing"
)

func TestCanonicalize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
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
		// .c is now preserved (different contract, not position alias)
		{"EURUSD.c", "EURUSD.C"},
		{"EURUSD.C", "EURUSD.C"},
		{"GBPJPY", "GBPJPY"},
		{"XAUUSD.raw", "XAUUSD"},
		{"EURUSD..m", "EURUSD."}, // .M stripped → trailing dot remains (only one suffix removed)
		{"A", "A"},               // too short (single char stays)
		{"AB", "AB"},             // too short
		{"ABCDEF.m", "ABCDEF"},   // len 9 > len(".M")+5=7 → strip ✓
		{"abcdef.m", "ABCDEF"},   // uppercase + strip
		// Shorter symbols: len(raw) ≤ len(suffix)+5 → no strip
		{"US30.PRO", "US30.PRO"}, // 8 ≤ 9 → no strip
		{"US30.I", "US30.I"},     // 6 ≤ 7 → no strip
		{"US30I", "US30I"},       // 5 ≤ 6 → no strip
		{"US30.STP", "US30.STP"}, // 8 ≤ 10 → no strip
		{"BRENT.C", "BRENT.C"},   // 7 ≤ 7 → no strip
	}

	for _, tt := range tests {
		got := Canonicalize(tt.input)
		if got != tt.want {
			t.Errorf("Canonicalize(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCanonicalizeNoDoubleStrip(t *testing.T) {
	// Symbol "EURUSD.m.ecn" — first suffix ".ECN" matches at pos 9
	// len("EURUSD.m.ecn")=13, len(".ECN")=4, 13 > 4+5=9 → true, strippable
	// Result: "EURUSD.m" — correct: only outermost suffix stripped
	got := Canonicalize("EURUSD.m.ECN")
	if got != "EURUSD.M" {
		t.Errorf("Canonicalize(EURUSD.m.ECN) = %q, want EURUSD.M", got)
	}
}
