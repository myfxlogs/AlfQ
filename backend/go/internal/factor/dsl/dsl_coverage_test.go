package dsl

import (
	"math"
	"testing"
)

// TestCompileMoreExprs hits additional code paths in compile.go / ref_delta.go / statistics.go
func TestCompileMoreExprs(t *testing.T) {
	fields := FieldIndex{Fields: map[string]int{"close": 0}}
	c := NewCompiler(fields, nil)

	tests := []struct {
		expr string
		n    int // number of bars to feed
	}{
		{`log(sma($close, 3))`, 10},
		{`sqrt(sma($close, 3))`, 10},
		{`pow(sma($close, 3), 2)`, 10},
		{`max(sma($close, 3), sma($close, 5))`, 10},
		{`min(sma($close, 3), 100)`, 10},
		{`(sma($close, 3) - sma($close, 5)) / sma($close, 5)`, 10},
		{`sma($close, 3) > sma($close, 5)`, 10},
		{`sma($close, 3) >= sma($close, 5)`, 10},
		{`sma($close, 3) < sma($close, 5)`, 10},
		{`sma($close, 3) <= sma($close, 5)`, 10},
		{`sma($close, 3) == sma($close, 5)`, 10},
		{`sma($close, 3) != sma($close, 5)`, 10},
		{`sma($close, 3) >= sma($close, 5)`, 10},
		{`sma($close, 3) > 0`, 10},
		{`sma($close, 3) < 1000`, 10},
	}

	for _, tt := range tests {
		op, err := c.Compile(tt.expr)
		if err != nil {
			t.Errorf("compile %q: %v", tt.expr, err)
			continue
		}
		for i := 0; i < tt.n; i++ {
			v := op.Eval(float64(100 + i))
			_ = v
		}
	}
}

func TestWMA(t *testing.T) {
	w := NewWMA(3)
	for i := 0; i < 8; i++ {
		v := w.Eval(float64(100 + i))
		if i < 2 && !math.IsNaN(v) {
			t.Errorf("WMA bar %d: expected NaN", i)
		}
		_ = v
	}
}

func TestCrossUp(t *testing.T) {
	cu := NewCrossUp()
	for i := 0; i < 10; i++ {
		a, b, _ := cu.Eval(float64(100 + i))
		_ = a
		_ = b
	}
}

func TestRefDeltaExtended(t *testing.T) {
	fields := FieldIndex{Fields: map[string]int{"close": 0}}
	c := NewCompiler(fields, nil)

	for _, expr := range []string{
		`ref($close, 3)`,
		`delta($close, 3)`,
	} {
		op, err := c.Compile(expr)
		if err != nil {
			t.Errorf("compile %q: %v", expr, err)
			continue
		}
		for i := 0; i < 10; i++ {
			_ = op.Eval(float64(100 + i*5))
		}
	}
}

func TestSTDAndVAR(t *testing.T) {
	s := NewSTD(5)
	for _, v := range []float64{1, 2, 3, 4, 5, 6, 7} {
		_ = s.Eval(v)
	}

	v := NewVAR(3)
	for _, val := range []float64{2, 4, 6, 8} {
		_ = v.Eval(val)
	}
}
