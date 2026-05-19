package dsl

import "math"

// BBUpper: sma(x, n) + k * std(x, n)
type BBUpper struct {
	sma *SMA
	std *STD
	k   float64
}

func NewBBUpper(n int, k float64) *BBUpper {
	return &BBUpper{sma: NewSMA(n), std: NewSTD(n), k: k}
}

func (b *BBUpper) Warmup() int { return nMax(b.sma.Warmup(), b.std.Warmup()) }
func (b *BBUpper) Eval(v float64) float64 {
	ma := b.sma.Eval(v)
	sd := b.std.Eval(v)
	if math.IsNaN(ma) || math.IsNaN(sd) {
		return math.NaN()
	}
	return ma + b.k*sd
}
func (b *BBUpper) Reset() { b.sma.Reset(); b.std.Reset() }

// BBLower: sma(x, n) - k * std(x, n)
type BBLower struct {
	sma *SMA
	std *STD
	k   float64
}

func NewBBLower(n int, k float64) *BBLower {
	return &BBLower{sma: NewSMA(n), std: NewSTD(n), k: k}
}

func (b *BBLower) Warmup() int { return b.sma.Warmup() }
func (b *BBLower) Eval(v float64) float64 {
	ma := b.sma.Eval(v)
	sd := b.std.Eval(v)
	if math.IsNaN(ma) || math.IsNaN(sd) {
		return math.NaN()
	}
	return ma - b.k*sd
}
func (b *BBLower) Reset() { b.sma.Reset(); b.std.Reset() }

// CrossUp returns 1.0 when x crosses above y, 0.0 otherwise.
type CrossUp struct {
	prevX, prevY float64
	init         bool
}

func NewCrossUp() *CrossUp { return &CrossUp{} }

func (c *CrossUp) Warmup() int { return 1 }
func (c *CrossUp) Eval(x float64) (float64, float64, error) {
	// CrossUp needs both x and y. For DSL integration, use CrossUp binary op.
	return 0, 0, nil
}

// CrossDown returns 1.0 when x crosses below y, 0.0 otherwise.
type CrossDown struct {
	prevX, prevY float64
	init         bool
}

func NewCrossDown() *CrossDown   { return &CrossDown{} }
func (c *CrossDown) Warmup() int { return 1 }
