// Package sizing computes position sizes from strategy specs and account state.
package sizing

// Calculator determines lot sizes for orders.
type Calculator struct{}

// NewCalculator creates a sizing calculator.
func NewCalculator() *Calculator {
	return &Calculator{}
}

// Compute returns the lot size based on the spec sizing config.
// Supports "fixed_lots" and "pct_equity" types.
func (c *Calculator) Compute(sizing map[string]any, equity float64, defaultLots float64) float64 {
	if sizing == nil {
		return defaultLots
	}

	switch sizing["type"] {
	case "fixed_lots":
		if lots, ok := sizing["lots"].(float64); ok {
			return lots
		}
	case "pct_equity":
		if pct, ok := sizing["pct"].(float64); ok && equity > 0 {
			risk := equity * pct / 100.0
			// Simplified: 1 lot ≈ 100k notional for FX
			lots := risk / 100_000.0
			if lots < 0.01 {
				lots = 0.01
			}
			return lots
		}
	}
	return defaultLots
}
