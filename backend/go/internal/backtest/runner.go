// Package backtest provides the Go-side backtest runner that calls the Python research CLI.
package backtest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Result holds the output of a backtest run.
type Result struct {
	StrategyID   string             `json:"strategy_id"`
	Status       string             `json:"status"` // "passed" | "failed"
	Correlation  float64            `json:"correlation"`
	DailyMADPct  float64            `json:"daily_mad_pct"`
	VecSharpe    float64            `json:"vec_sharpe"`
	EvSharpe     float64            `json:"ev_sharpe"`
	VecReturn    float64            `json:"vec_return"`
	EvReturn     float64            `json:"ev_return"`
	OverlapDays  int                `json:"overlap_days"`
	Error        string             `json:"error,omitempty"`
	RawOutput    string             `json:"-"`
}

// RunPythonBacktest executes the Python research backtest CLI for a given strategy spec.
// It writes the spec to a temp file, runs `uv run alfq-research backtest --spec <path>`,
// and parses the JSON output.
func RunPythonBacktest(ctx context.Context, specJSON []byte, researchDir string, timeout time.Duration) (*Result, error) {
	if researchDir == "" {
		researchDir = "research"
	}

	// Write spec to temp file
	tmpFile, err := writeTempSpec(specJSON)
	if err != nil {
		return nil, fmt.Errorf("backtest: temp spec: %w", err)
	}
	defer cleanupTemp(tmpFile)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx,
		"uv", "run", "python", "-m", "alfq_research.cli", "backtest",
		"--spec", tmpFile,
		"--output", "json",
	)
	cmd.Dir = researchDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return &Result{Status: "failed", Error: "timeout"}, nil
		}
		return &Result{
			Status:    "failed",
			Error:     fmt.Sprintf("backtest command failed: %v", err),
			RawOutput: string(output),
		}, nil
	}

	return parseResult(output)
}

// parseResult extracts the JSON result from the Python CLI output.
// The CLI prints a single JSON line with the result.
func parseResult(output []byte) (*Result, error) {
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	// Find the last JSON line
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "{") {
			var r Result
			if err := json.Unmarshal([]byte(line), &r); err == nil {
				r.RawOutput = string(output)
				return &r, nil
			}
		}
	}
	return &Result{
		Status:    "failed",
		Error:     "no JSON result found in output",
		RawOutput: string(output),
	}, nil
}

func writeTempSpec(data []byte) (string, error) {
	f, err := os.CreateTemp("", "alfq-backtest-spec-*.json")
	if err != nil {
		return "", err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return "", err
	}
	f.Close()
	return f.Name(), nil
}

func cleanupTemp(path string) {
	os.Remove(path)
}
