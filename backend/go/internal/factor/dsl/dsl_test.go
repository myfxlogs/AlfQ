package dsl

import (
	"math"
	"testing"
)

func TestLexer(t *testing.T) {
	toks, err := NewLexer(`ema($close, 20) / ema($close, 60) - 1`).LexAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(toks) < 5 {
		t.Fatalf("expected >= 5 tokens, got %d", len(toks))
	}
}

func TestParse(t *testing.T) {
	expr := `ema($close, 20) / ema($close, 60) - 1`
	node, err := Parse(expr)
	if err != nil {
		t.Fatal(err)
	}
	_ = node
}

func TestSMABasic(t *testing.T) {
	s := NewSMA(3)
	vals := []float64{1, 2, 3, 4, 5, 6}
	expected := []float64{math.NaN(), math.NaN(), 2, 3, 4, 5}
	for i, v := range vals {
		r := s.Eval(v)
		if math.IsNaN(expected[i]) != math.IsNaN(r) {
			t.Errorf("idx %d: expected NaN=%v, got NaN=%v", i, math.IsNaN(expected[i]), math.IsNaN(r))
		} else if !math.IsNaN(r) && math.Abs(r-expected[i]) > 1e-9 {
			t.Errorf("idx %d: expected %.6f, got %.6f", i, expected[i], r)
		}
	}
}

func TestEMA(t *testing.T) {
	e := NewEMA(3)
	for i := 0; i < 2; i++ {
		v := e.Eval(1.0)
		if !math.IsNaN(v) {
			t.Errorf("expected NaN during warmup, got %v", v)
		}
	}
}

func TestCompileAndEval(t *testing.T) {
	fields := FieldIndex{Fields: map[string]int{"close": 0}}
	c := NewCompiler(fields, nil)

	op, err := c.Compile(`sma($close, 3)`)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate bars: [100, 101, 102, 103, 104]
	bars := []float64{100, 101, 102, 103, 104}
	for i, v := range bars {
		r := op.Eval(v)
		switch i {
		case 0, 1:
			if !math.IsNaN(r) {
				t.Errorf("bar %d: expected NaN, got %.6f", i, r)
			}
		case 2:
			if math.Abs(r-101) > 1e-9 {
				t.Errorf("bar 2: expected 101, got %.6f", r)
			}
		case 3:
			if math.Abs(r-102) > 1e-9 {
				t.Errorf("bar 3: expected 102, got %.6f", r)
			}
		case 4:
			if math.Abs(r-103) > 1e-9 {
				t.Errorf("bar 4: expected 103, got %.6f", r)
			}
		}
	}
}

func TestValidation_Safety(t *testing.T) {
	err := ValidateExpression(`exec("ls")`, nil, nil)
	if err == nil {
		t.Error("expected rejection of dangerous token 'exec'")
	}
}

func TestCompile_UnknownField(t *testing.T) {
	fields := FieldIndex{Fields: map[string]int{"close": 0}}
	c := NewCompiler(fields, nil)
	_, err := c.Compile(`sma($open, 20)`)
	if err == nil {
		t.Error("expected error for unknown field $open")
	}
}
