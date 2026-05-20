// Package spec defines the strategy specification format used by quant-engine.
// Mirrors research/alfq_research/spec/strategy_spec.py.
package spec

// StrategySpec defines a complete trading strategy: canonical symbols, factor
// expressions, model reference, signal rule, and sizing.
type StrategySpec struct {
	Name             string            `json:"name" yaml:"name"`
	Version          string            `json:"version" yaml:"version"`
	CanonicalSymbols []string          `json:"canonical_symbols" yaml:"canonical_symbols"`
	Period           string            `json:"period" yaml:"period"`
	Factors          map[string]string `json:"factors" yaml:"factors"`
	SignalRule       string            `json:"signal_rule" yaml:"signal_rule"`
	ModelURI         string            `json:"model_uri,omitempty" yaml:"model_uri,omitempty"`
	ModelInputs      []string          `json:"model_inputs,omitempty" yaml:"model_inputs,omitempty"`
	Sizing           map[string]any    `json:"sizing,omitempty" yaml:"sizing,omitempty"`
	RiskLimits       map[string]float64 `json:"risk_limits,omitempty" yaml:"risk_limits,omitempty"`
	Description      string            `json:"description,omitempty" yaml:"description,omitempty"`
}

// Validate checks the spec and returns a list of issues (empty = valid).
func (s *StrategySpec) Validate() []string {
	var issues []string
	if s.Name == "" {
		issues = append(issues, "name is required")
	}
	if len(s.CanonicalSymbols) == 0 {
		issues = append(issues, "canonical_symbols is required")
	}
	if s.SignalRule == "" && s.ModelURI == "" {
		issues = append(issues, "either signal_rule or model_uri is required")
	}
	if s.ModelURI != "" && len(s.ModelInputs) == 0 {
		issues = append(issues, "model_inputs is required when model_uri is set")
	}
	switch s.Period {
	case "1m", "5m", "15m", "30m", "1h", "4h", "1d", "1w", "":
	default:
		issues = append(issues, "unknown period: "+s.Period)
	}
	return issues
}

// IsValid returns true if the spec passes validation.
func (s *StrategySpec) IsValid() bool {
	return len(s.Validate()) == 0
}
