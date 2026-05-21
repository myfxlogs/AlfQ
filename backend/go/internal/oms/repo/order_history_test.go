package repo

import (
	"testing"

	"github.com/alfq/backend/go/internal/mdgateway/adapter/mtapi"
)

func TestNewHistoryOrderRepo(t *testing.T) {
	// Test with nil pool - should still create the struct
	r := NewHistoryOrderRepo(nil)
	if r == nil {
		t.Fatal("NewHistoryOrderRepo returned nil")
	}
}

func TestToHistoryOrder(t *testing.T) {
	info := &mtapi.HistoryOrderInfo{
		Ticket:     12345,
		Symbol:     "EURUSD",
		Type:       "buy",
		Lots:       0.1,
		OpenPrice:  1.1000,
		ClosePrice: 1.1050,
		Profit:     50.0,
		Swap:       -0.5,
		Commission: 1.0,
		OpenTime:   "2024-01-01T10:00:00Z",
		CloseTime:  "2024-01-01T11:00:00Z",
	}

	order := ToHistoryOrder("tenant-123", "account-456", info, "closed")

	if order.TenantID != "tenant-123" {
		t.Fatalf("TenantID = %s, want tenant-123", order.TenantID)
	}
	if order.AccountID != "account-456" {
		t.Fatalf("AccountID = %s, want account-456", order.AccountID)
	}
	if order.Ticket != 12345 {
		t.Fatalf("Ticket = %d, want 12345", order.Ticket)
	}
	if order.Symbol != "EURUSD" {
		t.Fatalf("Symbol = %s, want EURUSD", order.Symbol)
	}
	if order.Side != "buy" {
		t.Fatalf("Side = %s, want buy", order.Side)
	}
	if order.Lots != 0.1 {
		t.Fatalf("Lots = %f, want 0.1", order.Lots)
	}
	if order.State != "closed" {
		t.Fatalf("State = %s, want closed", order.State)
	}
	if order.CloseTime == nil {
		t.Fatal("CloseTime should not be nil")
	}
}

func TestToHistoryOrder_EmptyCloseTime(t *testing.T) {
	info := &mtapi.HistoryOrderInfo{
		Ticket:    12345,
		Symbol:    "EURUSD",
		Type:      "buy",
		Lots:      0.1,
		OpenPrice: 1.1000,
		OpenTime:  "2024-01-01T10:00:00Z",
		CloseTime: "",
	}

	order := ToHistoryOrder("tenant-123", "account-456", info, "closed")

	if order.CloseTime != nil {
		t.Fatal("CloseTime should be nil for empty close time")
	}
}

func TestToHistoryOrder_DefaultState(t *testing.T) {
	info := &mtapi.HistoryOrderInfo{
		Ticket:    12345,
		Symbol:    "EURUSD",
		Type:      "buy",
		Lots:      0.1,
		OpenPrice: 1.1000,
		OpenTime:  "2024-01-01T10:00:00Z",
	}

	order := ToHistoryOrder("tenant-123", "account-456", info, "")

	if order.State != "closed" {
		t.Fatalf("State = %s, want 'closed' as default", order.State)
	}
}

func TestToHistoryOrder_InvalidTimeFormat(t *testing.T) {
	info := &mtapi.HistoryOrderInfo{
		Ticket:    12345,
		Symbol:    "EURUSD",
		Type:      "buy",
		Lots:      0.1,
		OpenPrice: 1.1000,
		OpenTime:  "invalid-time",
		CloseTime: "also-invalid",
	}

	order := ToHistoryOrder("tenant-123", "account-456", info, "closed")

	// Should not panic, just return zero time for invalid formats
	if !order.OpenTime.IsZero() {
		t.Fatal("OpenTime should be zero for invalid format")
	}
}

func TestToHistoryOrder_RawPayload(t *testing.T) {
	info := &mtapi.HistoryOrderInfo{
		Ticket:    12345,
		Symbol:    "EURUSD",
		Type:      "buy",
		Lots:      0.1,
		OpenPrice: 1.1000,
		OpenTime:  "2024-01-01T10:00:00Z",
		CloseTime: "2024-01-01T11:00:00Z",
	}

	order := ToHistoryOrder("tenant-123", "account-456", info, "closed")

	if order.RawPayload == nil {
		t.Fatal("RawPayload should not be nil")
	}
	if order.RawPayload["open_time_str"] != "2024-01-01T10:00:00Z" {
		t.Fatalf("RawPayload open_time_str = %s, want 2024-01-01T10:00:00Z", order.RawPayload["open_time_str"])
	}
	if order.RawPayload["close_time_str"] != "2024-01-01T11:00:00Z" {
		t.Fatalf("RawPayload close_time_str = %s, want 2024-01-01T11:00:00Z", order.RawPayload["close_time_str"])
	}
}
