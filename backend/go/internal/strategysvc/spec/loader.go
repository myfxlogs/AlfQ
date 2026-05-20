// Package spec — strategy spec loader from YAML/JSON files.
package spec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadFile loads a StrategySpec from a YAML or JSON file.
func LoadFile(path string) (*StrategySpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("spec load: %w", err)
	}
	ext := filepath.Ext(path)
	switch ext {
	case ".yaml", ".yml":
		return parseYAML(data)
	case ".json":
		return parseJSON(data)
	default:
		return nil, fmt.Errorf("spec load: unsupported format %q", ext)
	}
}

// LoadDir loads all spec files (*.yaml, *.yml, *.json) from a directory.
func LoadDir(dir string) ([]*StrategySpec, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("spec load dir: %w", err)
	}
	var specs []*StrategySpec
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext != ".yaml" && ext != ".yml" && ext != ".json" {
			continue
		}
		s, err := LoadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		if !s.IsValid() {
			return nil, fmt.Errorf("spec %q is invalid: %v", e.Name(), s.Validate())
		}
		specs = append(specs, s)
	}
	return specs, nil
}

func parseYAML(data []byte) (*StrategySpec, error) {
	var s StrategySpec
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("spec yaml: %w", err)
	}
	return &s, nil
}

func parseJSON(data []byte) (*StrategySpec, error) {
	var s StrategySpec
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("spec json: %w", err)
	}
	return &s, nil
}
