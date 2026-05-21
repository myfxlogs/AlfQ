package factorsvc

import (
	"testing"
)

func TestNewEngine(t *testing.T) {
	cfg := Config{
		Factors: []FactorDef{
			{Name: "test", Expression: "close", Symbols: []string{"EURUSD"}},
		},
		NatsURL: "nats://localhost:4222",
	}
	e := NewEngine(cfg)
	if e == nil {
		t.Fatal("NewEngine returned nil")
	}
}

func TestFactorDef_Fields(t *testing.T) {
	fd := FactorDef{
		Name:       "test",
		Expression: "close",
		Symbols:    []string{"EURUSD", "GBPJPY"},
	}
	if fd.Name != "test" {
		t.Fatalf("expected test, got %s", fd.Name)
	}
	if len(fd.Symbols) != 2 {
		t.Fatalf("expected 2 symbols, got %d", len(fd.Symbols))
	}
}
