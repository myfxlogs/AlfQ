package assistantsvc

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExtractBearer(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"valid", "Bearer abc123", "abc123"},
		{"empty", "", ""},
		{"no prefix", "abc123", ""},
		{"lowercase", "bearer abc123", ""},
		{"with spaces", "Bearer  abc123  ", " abc123  "},
	}
	for _, tt := range tests {
		r, _ := http.NewRequest("GET", "/", nil)
		if tt.header != "" {
			r.Header.Set("Authorization", tt.header)
		}
		got := extractBearer(r)
		if got != tt.want {
			t.Errorf("%s: extractBearer() = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestWriteJSONResp(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSONResp(w, 200, map[string]string{"ok": "true"})
	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"ok"`) {
		t.Errorf("body should contain ok: %s", w.Body.String())
	}

	w2 := httptest.NewRecorder()
	writeJSONResp(w2, 500, map[string]string{"error": "fail"})
	if w2.Code != 500 {
		t.Errorf("expected 500, got %d", w2.Code)
	}
}

func TestEstimateCost(t *testing.T) {
	tests := []struct {
		provider  string
		model     string
		tokensIn  int
		tokensOut int
		wantMin   int
		wantMax   int
	}{
		{"openai", "gpt-4o", 1000, 500, 0, 2},
		{"openai", "gpt-4", 1000, 500, 3, 8},
		{"openai", "gpt-3.5-turbo", 1000, 500, 0, 1},
		{"anthropic", "claude-sonnet-4-20250514", 1000, 500, 0, 2},
		{"openai", "gpt-4o", 0, 0, 0, 0},
	}
	for _, tt := range tests {
		cost := estimateCost(tt.provider, tt.model, tt.tokensIn, tt.tokensOut)
		if cost < tt.wantMin || cost > tt.wantMax {
			t.Errorf("estimateCost(%s, %s, %d, %d) = %d, want [%d,%d]",
				tt.provider, tt.model, tt.tokensIn, tt.tokensOut, cost, tt.wantMin, tt.wantMax)
		}
	}
}

func TestEstimateCostZero(t *testing.T) {
	cost := estimateCost("openai", "gpt-4o", 0, 0)
	if cost != 0 {
		t.Errorf("zero tokens should cost 0, got %d", cost)
	}
}
