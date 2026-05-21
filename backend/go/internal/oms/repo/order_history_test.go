package repo

import (
	"testing"

	"github.com/alfq/backend/go/internal/mdgateway/adapter/mtapi"
)

func TestNewHistoryOrderRepo(t *testing.T) {
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
		Swap:       -1.0,
		Commission: 0.5,
		OpenTime:   "2024-01-01T00:00:00Z",
		CloseTime:  "2024-01-01T00:05:00Z",
	}
	order := ToHistoryOrder("tenant-1", "account-1", info, "closed")
	if order == nil {
		t.Fatal("ToHistoryOrder returned nil")
	}
	if order.TenantID != "tenant-1" {
		t.Fatalf("expected tenant-1, got %s", order.TenantID)
	}
	if order.AccountID != "account-1" {
		t.Fatalf("expected account-1, got %s", order.AccountID)
	}
	if order.Ticket != 12345 {
		t.Fatalf("expected 12345, got %d", order.Ticket)
	}
	if order.State != "closed" {
		t.Fatalf("expected closed, got %s", order.State)
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
