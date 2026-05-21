// Package chmigrate — migration runner tests.
package chmigrate

import (
	"testing"
)

func TestRun_Skipped(t *testing.T) {
	// Skip this test since Run requires a real ClickHouse connection
	// and panics on nil connection
	t.Skip("Run requires real ClickHouse connection")
}

func TestMustRun_Skipped(t *testing.T) {
	// Skip this test since MustRun panics on error and we don't have a real ClickHouse connection
	t.Skip("MustRun requires real ClickHouse connection")
}
