package spec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStrategySpecValidate(t *testing.T) {
	tests := []struct {
		name    string
		spec    StrategySpec
		wantErr bool
	}{
		{
			name:    "empty",
			spec:    StrategySpec{},
			wantErr: true,
		},
		{
			name: "valid minimal",
			spec: StrategySpec{
				Name:             "test",
				CanonicalSymbols: []string{"EURUSD"},
				SignalRule:       "sma20 > sma60 ? 1 : -1",
				Period:           "1h",
			},
			wantErr: false,
		},
		{
			name: "valid with model",
			spec: StrategySpec{
				Name:             "ml_strat",
				CanonicalSymbols: []string{"EURUSD"},
				ModelURI:         "s3://bucket/model.onnx",
				ModelInputs:      []string{"f1", "f2"},
				Period:           "1d",
			},
			wantErr: false,
		},
		{
			name: "model without inputs",
			spec: StrategySpec{
				Name:             "bad_ml",
				CanonicalSymbols: []string{"EURUSD"},
				ModelURI:         "s3://bucket/model.onnx",
			},
			wantErr: true,
		},
		{
			name: "bad period",
			spec: StrategySpec{
				Name:             "bad_period",
				CanonicalSymbols: []string{"EURUSD"},
				SignalRule:       "1",
				Period:           "2h",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.spec.IsValid(); got == tt.wantErr {
				t.Errorf("IsValid() = %v, want %v, issues: %v", got, !tt.wantErr, tt.spec.Validate())
			}
		})
	}
}

func TestLoadFile_YAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	data := `
name: momentum
version: "1.0"
canonical_symbols:
  - EURUSD
  - GBPUSD
period: 1h
factors:
  mom: "ema($close, 20) / ema($close, 60) - 1"
signal_rule: "mom > 0 ? 1 : -1"
sizing:
  type: fixed_lots
  lots: 0.1
`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	spec, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Name != "momentum" {
		t.Errorf("name = %q, want momentum", spec.Name)
	}
	if len(spec.CanonicalSymbols) != 2 {
		t.Errorf("symbols = %d, want 2", len(spec.CanonicalSymbols))
	}
	if spec.Factors["mom"] == "" {
		t.Error("factor mom not loaded")
	}
}

func TestLoadFile_JSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	data := `{"name":"json_test","canonical_symbols":["XAUUSD"],"signal_rule":"1","period":"1d"}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	spec, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Name != "json_test" {
		t.Errorf("name = %q", spec.Name)
	}
}

func TestLoadDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.yaml"), []byte(`name: a
canonical_symbols: [EURUSD]
signal_rule: "1"
period: 1h`), 0o644)
	os.WriteFile(filepath.Join(dir, "b.yaml"), []byte(`name: b
canonical_symbols: [GBPUSD]
signal_rule: "-1"
period: 1d`), 0o644)

	specs, err := LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 2 {
		t.Errorf("got %d specs, want 2", len(specs))
	}
}


