package mdgateway

import (
	"testing"
)

func TestFirstN(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		n        int
		expected []string
	}{
		{"less than n", []string{"a", "b"}, 5, []string{"a", "b"}},
		{"equal to n", []string{"a", "b"}, 2, []string{"a", "b"}},
		{"more than n", []string{"a", "b", "c", "d"}, 2, []string{"a", "b"}},
		{"empty", []string{}, 5, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := firstN(tt.input, tt.n)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d items, got %d", len(tt.expected), len(result))
			}
		})
	}
}

func TestSplitHostPort(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		defaultPort string
		wantHost    string
		wantPort    string
	}{
		{"host:port", "example.com:8080", "443", "example.com", "8080"},
		{"host only", "example.com", "443", "example.com", "443"},
		{"empty", "", "443", "", "443"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port := splitHostPort(tt.input, tt.defaultPort)
			if host != tt.wantHost {
				t.Fatalf("expected host %s, got %s", tt.wantHost, host)
			}
			if port != tt.wantPort {
				t.Fatalf("expected port %s, got %s", tt.wantPort, port)
			}
		})
	}
}

func TestExtractBrokerID(t *testing.T) {
	tests := []struct {
		key          string
		wantBrokerID string
		wantLogin    string
	}{
		{"d6ad41cd-12345678", "d6ad41cd", "12345678"},
		{"abd5a77d-87654321", "abd5a77d", "87654321"},
		{"nodash", "nodash", "nodash"},
		{"b3f8a91c-7e2d-4a1b-9c6d-5f0e8a2b3d4f-888888", "b3f8a91c-7e2d-4a1b-9c6d-5f0e8a2b3d4f", "888888"},
		{"a-b-c-d-99999", "a-b-c-d", "99999"},
		{"", "", ""},
	}

	for _, tt := range tests {
		gotBrokerID, gotLogin := extractBrokerID(tt.key)
		if gotBrokerID != tt.wantBrokerID {
			t.Errorf("extractBrokerID(%q) brokerID = %q, want %q", tt.key, gotBrokerID, tt.wantBrokerID)
		}
		if gotLogin != tt.wantLogin {
			t.Errorf("extractBrokerID(%q) login = %q, want %q", tt.key, gotLogin, tt.wantLogin)
		}
	}
}
