package flags

import (
	"context"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	c := NewClient()
	if c == nil {
		t.Fatal("nil client")
	}
}

func TestRegisterAndBool(t *testing.T) {
	c := NewClient()
	c.Register(FlagDefinition{
		Key: "feature_x", Type: FlagTypeBool, DefaultVal: true,
		Rollout: Rollout{Rules: []RolloutRule{{Value: true}}},
	})
	if !c.Bool(context.Background(), "feature_x", false) {
		t.Fatal("expected true from rule match")
	}
}

func TestBoolDefault(t *testing.T) {
	c := NewClient()
	if !c.Bool(context.Background(), "missing", true) {
		t.Fatal("expected default")
	}
}

func TestIntFlag(t *testing.T) {
	c := NewClient()
	c.Register(FlagDefinition{
		Key: "rate", Type: FlagTypeInt, DefaultVal: 10,
		Rollout: Rollout{Rules: []RolloutRule{{Value: 5}}},
	})
	if c.Int(context.Background(), "rate", 0) != 5 {
		t.Fatal("expected 5")
	}
}

func TestStringFlag(t *testing.T) {
	c := NewClient()
	c.Register(FlagDefinition{
		Key: "mode", Type: FlagTypeString, DefaultVal: "a",
		Rollout: Rollout{Rules: []RolloutRule{{Value: "b"}}},
	})
	if c.String(context.Background(), "mode", "") != "b" {
		t.Fatal("expected b")
	}
}

func TestReload(t *testing.T) {
	c := NewClient()
	c.Reload([]FlagDefinition{{Key: "f1", Type: FlagTypeBool, DefaultVal: true}})
	if c.String(context.Background(), "f1", "x") != "x" {
		t.Fatal("expected default string")
	}
}

func TestList(t *testing.T) {
	c := NewClient()
	c.Register(FlagDefinition{Key: "a", Type: FlagTypeBool})
	c.Register(FlagDefinition{Key: "b", Type: FlagTypeBool})
	l := c.List()
	if len(l) != 2 {
		t.Fatalf("expected 2, got %d", len(l))
	}
}
