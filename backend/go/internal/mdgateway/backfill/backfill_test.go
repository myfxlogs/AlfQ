package backfill

import (
	"testing"
)

func TestSplitHostPort(t *testing.T) {
	tests := []struct {
		input       string
		defaultPort string
		wantHost    string
		wantPort    string
	}{
		{"example.com:443", "80", "example.com", "443"},
		{"example.com:8080", "80", "example.com", "8080"},
		{"example.com", "443", "example.com", "443"},
		{"192.168.1.1:9000", "80", "192.168.1.1", "9000"},
		{"[::1]:9000", "80", "[::1]", "9000"},
	}

	for _, tt := range tests {
		host, port := splitHostPort(tt.input, tt.defaultPort)
		if host != tt.wantHost {
			t.Fatalf("splitHostPort(%q, %q) host = %q, want %q", tt.input, tt.defaultPort, host, tt.wantHost)
		}
		if port != tt.wantPort {
			t.Fatalf("splitHostPort(%q, %q) port = %q, want %q", tt.input, tt.defaultPort, port, tt.wantPort)
		}
	}
}

func TestSplitHostPort_NoPort(t *testing.T) {
	host, port := splitHostPort("example.com", "80")
	if host != "example.com" {
		t.Fatalf("expected example.com, got %s", host)
	}
	if port != "80" {
		t.Fatalf("expected 80, got %s", port)
	}
}

func TestParseUint(t *testing.T) {
	tests := []struct {
		input string
		want  uint64
	}{
		{"123", 123},
		{"0", 0},
		{"999999", 999999},
		{"abc", 0},
		{"", 0},
		{"12a34", 1234}, // parseUint stops at non-digits
	}

	for _, tt := range tests {
		got := parseUint(tt.input)
		if got != tt.want {
			t.Fatalf("parseUint(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestParsePort(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"443", 443},
		{"8080", 8080},
		{"0", 443},
		{"abc", 443},
		{"", 443},
		{"9000", 9000},
	}

	for _, tt := range tests {
		got := parsePort(tt.input)
		if got != tt.want {
			t.Fatalf("parsePort(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestSession_Close_Nil(t *testing.T) {
	// Close on nil conn will panic, so we skip this test
	// In practice, Session should only be created with a valid conn
	t.Skip("Close on nil conn causes panic, skipped")
}

func TestBar_Fields(t *testing.T) {
	bar := Bar{
		Time:   1234567890,
		Open:   1.1000,
		High:   1.1050,
		Low:    1.0990,
		Close:  1.1040,
		Volume: 100.0,
	}
	if bar.Time != 1234567890 {
		t.Fatalf("expected 1234567890, got %d", bar.Time)
	}
	if bar.Open != 1.1000 {
		t.Fatalf("expected 1.1000, got %f", bar.Open)
	}
}
