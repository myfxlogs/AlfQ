package backtest

import (
	"testing"
)

func TestParseResult_ValidJSON(t *testing.T) {
	output := []byte(`{"strategy_id":"s1","status":"passed","correlation":0.5,"daily_mad_pct":1.2,"vec_sharpe":1.5,"ev_sharpe":1.3,"vec_return":2.1,"ev_return":1.9,"overlap_days":30}`)
	r, err := parseResult(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Status != "passed" {
		t.Fatalf("expected status passed, got %s", r.Status)
	}
	if r.StrategyID != "s1" {
		t.Fatalf("expected strategy_id s1, got %s", r.StrategyID)
	}
	if r.Correlation != 0.5 {
		t.Fatalf("expected correlation 0.5, got %f", r.Correlation)
	}
}

func TestParseResult_MultilineOutput(t *testing.T) {
	output := []byte("some log line\n" +
		`{"strategy_id":"s2","status":"failed","error":"something went wrong"}` + "\n")
	r, err := parseResult(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Status != "failed" {
		t.Fatalf("expected status failed, got %s", r.Status)
	}
	if r.Error != "something went wrong" {
		t.Fatalf("expected error message, got %s", r.Error)
	}
}

func TestParseResult_NoJSON(t *testing.T) {
	output := []byte("just plain text output")
	r, err := parseResult(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Status != "failed" {
		t.Fatalf("expected status failed, got %s", r.Status)
	}
	if r.Error != "no JSON result found in output" {
		t.Fatalf("expected no JSON error, got %s", r.Error)
	}
}

func TestParseResult_InvalidJSON(t *testing.T) {
	output := []byte("{not valid json}")
	r, err := parseResult(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Status != "failed" {
		t.Fatalf("expected status failed, got %s", r.Status)
	}
}

func TestWriteTempSpec(t *testing.T) {
	path, err := writeTempSpec([]byte(`{"test": true}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}
	cleanupTemp(path)
}

func TestCleanupTemp(t *testing.T) {
	// cleanupTemp should not panic on non-existent path
	cleanupTemp("/tmp/non-existent-file-for-testing-12345")
}
