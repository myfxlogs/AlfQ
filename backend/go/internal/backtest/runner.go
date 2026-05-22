// Package backtest provides the Go-side backtest runner that calls the Python research CLI.
package backtest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Result holds the output of a backtest run.
type Result struct {
	StrategyID  string  `json:"strategy_id"`
	Status      string  `json:"status"` // "passed" | "failed"
	Correlation float64 `json:"correlation"`
	DailyMADPct float64 `json:"daily_mad_pct"`
	VecSharpe   float64 `json:"vec_sharpe"`
	EvSharpe    float64 `json:"ev_sharpe"`
	VecReturn   float64 `json:"vec_return"`
	EvReturn    float64 `json:"ev_return"`
	OverlapDays int     `json:"overlap_days"`
	Error       string  `json:"error,omitempty"`
	RawOutput   string  `json:"-"`
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
	_ = os.Remove(path)
}

// backtestRunnerAddr is the address of the backtest-runner HTTP service.
// Defaults to the Docker compose internal service name.
var backtestRunnerAddr = envOrDefault("BACKTEST_RUNNER_ADDR", "http://backtest-runner:9009")

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// RunViaService sends a backtest spec to the backtest-runner HTTP service.
// This replaces the exec.Command approach when the backtest-runner container is available.
func RunViaService(ctx context.Context, spec map[string]any) (*Result, error) {
	body, err := json.Marshal(map[string]any{"spec": spec})
	if err != nil {
		return nil, fmt.Errorf("backtest: marshal spec: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		backtestRunnerAddr+"/run", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("backtest: request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("backtest: call runner: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB max

	if resp.StatusCode != http.StatusOK {
		return &Result{Status: "failed", Error: string(respBody)}, nil
	}

	var wrapper struct {
		Stdout string `json:"stdout"`
		Stderr string `json:"stderr"`
	}
	if err := json.Unmarshal(respBody, &wrapper); err != nil {
		return &Result{Status: "failed", Error: fmt.Sprintf("unmarshal response: %v", err)}, nil
	}

	return parseResult([]byte(wrapper.Stdout))
}
