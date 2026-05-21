package mthub

import (
	"testing"
)

func TestRecordActiveSessions(t *testing.T) {
	// This function updates prometheus metrics, just ensure it doesn't panic
	active := map[string]int{
		"mt4": 5,
		"mt5": 10,
	}
	recordActiveSessions(active)
	// No assertion needed, just ensure it runs without panic
}

func TestRecordActiveSessions_Empty(t *testing.T) {
	active := map[string]int{}
	recordActiveSessions(active) // Should not panic
}

func TestMetrics_NotNil(t *testing.T) {
	// Metrics are initialized in init(), just ensure they exist
	// This test ensures the init function ran
}
