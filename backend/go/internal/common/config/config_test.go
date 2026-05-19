package config_test

import (
	"testing"

	"github.com/alfq/backend/go/internal/common/config"
)

func TestDefaults(t *testing.T) {
	cfg := config.Defaults()
	if cfg.Server.Listen != ":9000" {
		t.Fatalf("Listen: %s", cfg.Server.Listen)
	}
	if cfg.Log.Level != "info" {
		t.Fatalf("Level: %s", cfg.Log.Level)
	}
}

func TestLoadNilCfg(t *testing.T) {
	_, err := config.Load("nonexistent.yaml", nil)
	if err != nil {
		t.Fatalf("Load nil cfg: %v", err)
	}
}

func TestLoadMissingFile(t *testing.T) {
	cfg := config.Defaults()
	_, err := config.Load("nonexistent.yaml", cfg)
	if err != nil {
		t.Fatalf("Load missing: %v", err)
	}
}
