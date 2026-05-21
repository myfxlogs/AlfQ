package mtapi

import (
	"testing"
)

func TestAccountInfo_Fields(t *testing.T) {
	ai := AccountInfo{
		Balance:     1000.0,
		Equity:      1100.0,
		Margin:      100.0,
		FreeMargin:  900.0,
		MarginLevel: 10.0,
		Profit:      100.0,
		Currency:    "USD",
		Leverage:    100,
	}
	if ai.Balance != 1000.0 {
		t.Fatalf("expected 1000.0, got %f", ai.Balance)
	}
	if ai.Currency != "USD" {
		t.Fatalf("expected USD, got %s", ai.Currency)
	}
}

func TestPositionInfo_Fields(t *testing.T) {
	pi := PositionInfo{
		Ticket:    12345,
		Symbol:    "EURUSD",
		Type:      "buy",
		Lots:      0.1,
		OpenPrice: 1.1000,
		Profit:    10.0,
	}
	if pi.Ticket != 12345 {
		t.Fatalf("expected 12345, got %d", pi.Ticket)
	}
	if pi.Symbol != "EURUSD" {
		t.Fatalf("expected EURUSD, got %s", pi.Symbol)
	}
}

func TestHistoryOrderInfo_Fields(t *testing.T) {
	h := HistoryOrderInfo{
		Ticket:  12345,
		Symbol:  "EURUSD",
		Type:    "buy",
		Lots:    0.1,
		Profit:  10.0,
	}
	if h.Ticket != 12345 {
		t.Fatalf("expected 12345, got %d", h.Ticket)
	}
}

func TestBrokerMatch_Fields(t *testing.T) {
	bm := BrokerMatch{
		Company: "Test Broker",
		Servers: []ServerEntry{
			{Name: "Server1", Access: "1.2.3.4:443"},
		},
	}
	if bm.Company != "Test Broker" {
		t.Fatalf("expected Test Broker, got %s", bm.Company)
	}
	if len(bm.Servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(bm.Servers))
	}
}

func TestServerEntry_Fields(t *testing.T) {
	se := ServerEntry{
		Name:   "TestServer",
		Access: "1.2.3.4:443",
	}
	if se.Name != "TestServer" {
		t.Fatalf("expected TestServer, got %s", se.Name)
	}
	if se.Access != "1.2.3.4:443" {
		t.Fatalf("expected 1.2.3.4:443, got %s", se.Access)
	}
}

func TestParseUint(t *testing.T) {
	tests := []struct {
		input string
		want  uint64
	}{
		{"123", 123},
		{"456", 456},
		{"0", 0},
		{"abc", 0}, // non-digits are ignored
	}
	for _, tt := range tests {
		if got := parseUint(tt.input); got != tt.want {
			t.Fatalf("parseUint(%s) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestParsePort(t *testing.T) {
	tests := []struct {
		input string
		want  int32
	}{
		{"443", 443},
		{"8080", 8080},
		{"0", 443}, // 0 defaults to 443
		{"abc", 443}, // non-digits default to 443
	}
	for _, tt := range tests {
		if got := parsePort(tt.input); got != tt.want {
			t.Fatalf("parsePort(%s) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestSplitHostPort(t *testing.T) {
	tests := []struct {
		input       string
		defaultPort string
		wantHost    string
		wantPort    string
	}{
		{"example.com:443", "80", "example.com", "443"},
		{"example.com", "80", "example.com", "80"},
		{"localhost:8080", "80", "localhost", "8080"},
	}
	for _, tt := range tests {
		host, port := splitHostPort(tt.input, tt.defaultPort)
		if host != tt.wantHost || port != tt.wantPort {
			t.Fatalf("splitHostPort(%s, %s) = (%s, %s), want (%s, %s)", tt.input, tt.defaultPort, host, port, tt.wantHost, tt.wantPort)
		}
	}
}
