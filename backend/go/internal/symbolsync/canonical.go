// Package symbolsync — canonical symbol name normalisation.
package symbolsync

import "strings"

// Canonicalize converts a broker-specific symbol name to canonical form.
// Priority:
//  1. Lookup symbol_canonical_overrides (handled by caller)
//  2. Strip common suffixes: .m, m, .ecn, .raw, .pro, .i, i, .stp, .c
//  3. Uppercase
func Canonicalize(raw string) string {
	raw = strings.ToUpper(raw)
	suffixes := []string{".M", "M", ".ECN", ".RAW", ".PRO", ".I", "I", ".STP", ".C"}
	for _, s := range suffixes {
		if strings.HasSuffix(raw, s) && len(raw) > len(s)+5 {
			return strings.TrimSuffix(raw, s)
		}
	}
	return raw
}
