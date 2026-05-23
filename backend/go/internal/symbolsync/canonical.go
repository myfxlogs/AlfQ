// Package symbolsync — canonical symbol name normalisation.
package symbolsync

import "strings"

// Canonicalize converts a broker-specific symbol name to canonical form.
// Priority:
//  1. Lookup symbol_canonical_overrides (handled by caller)
//  2. Strip common suffixes: .m, m, .ecn, .raw, .pro, .i, i, .stp, .x, #, !
//  3. Uppercase
//
// Note: .c suffix is NOT stripped because EURUSDc etc. are different contracts,
// not position aliases. Those go through partial=true + manual review.
func Canonicalize(raw string) string {
	raw = strings.ToUpper(raw)
	// Ordered by specificity: longer/dotted suffixes first
	suffixes := []string{".ECN", ".RAW", ".PRO", ".STP", ".M", ".I", ".X"}
	for _, s := range suffixes {
		if strings.HasSuffix(raw, s) && len(raw) > len(s)+4 {
			return strings.TrimSuffix(raw, s)
		}
	}
	// Bare suffixes (no dot) - only strip if the base is long enough to be a real symbol
	bareSuffixes := []string{"M", "I"}
	for _, s := range bareSuffixes {
		if strings.HasSuffix(raw, s) && len(raw) > len(s)+5 {
			// Don't strip if the character before is a digit (e.g. US30 is not US3 + 0)
			prev := raw[len(raw)-len(s)-1]
			if prev >= 'A' && prev <= 'Z' {
				return strings.TrimSuffix(raw, s)
			}
		}
	}
	// Strip trailing special chars: #, ! (e.g. BTCUSD# → BTCUSD)
	if last := raw[len(raw)-1]; last == '#' || last == '!' {
		if len(raw) > 6 {
			return raw[:len(raw)-1]
		}
	}
	return raw
}
