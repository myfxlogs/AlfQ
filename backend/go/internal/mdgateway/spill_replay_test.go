package mdgateway

import (
	"testing"
)

func TestNewSpillReplay(t *testing.T) {
	s := NewSpillReplay("/tmp/spill", nil, nil)
	if s == nil {
		t.Fatal("NewSpillReplay returned nil")
	}
	if s.dir != "/tmp/spill" {
		t.Fatalf("expected /tmp/spill, got %s", s.dir)
	}
}

func TestSpillReplay_Replay_EmptyDir(t *testing.T) {
	s := NewSpillReplay("", nil, nil)
	count, err := s.Replay(nil)
	if err != nil {
		t.Fatalf("Replay with empty dir should not error, got %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0, got %d", count)
	}
}
