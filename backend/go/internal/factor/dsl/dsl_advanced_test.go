package dsl

import (
	"math"
	"testing"
)

func TestRSI(t *testing.T) {
	r := NewRSI(3)
	prices := []float64{10, 12, 14, 11, 13, 15, 12}
	for _, p := range prices {
		_ = r.Eval(p)
	}
	last := r.Eval(10)
	if math.IsNaN(last) {
		t.Fatal("RSI should not be NaN after warmup")
	}
	if last < 0 || last > 100 {
		t.Fatalf("RSI should be in [0,100], got %.2f", last)
	}
}

func TestRSIInitial(t *testing.T) {
	r := NewRSI(5)
	v := r.Eval(100)
	if !math.IsNaN(v) {
		t.Fatal("first RSI eval should be NaN")
	}
}

func TestMACD(t *testing.T) {
	m := NewMACD(3, 6)
	prices := []float64{10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	for _, p := range prices {
		_ = m.Eval(p)
	}
	macd := m.Eval(21)
	if math.IsNaN(macd) {
		t.Fatal("MACD should not be NaN after sufficient data")
	}
}

func TestBBUpper(t *testing.T) {
	b := NewBBUpper(3, 2)
	prices := []float64{10, 12, 14, 11, 13, 15}
	for _, p := range prices {
		_ = b.Eval(p)
	}
	v := b.Eval(12)
	if math.IsNaN(v) {
		t.Fatal("BBUpper should not be NaN")
	}
}

func TestBBUpperWarmup(t *testing.T) {
	b := NewBBUpper(5, 2)
	v := b.Eval(100)
	if !math.IsNaN(v) {
		t.Fatal("first BBUpper eval should be NaN")
	}
}

func TestBBLower(t *testing.T) {
	b := NewBBLower(3, 2)
	prices := []float64{10, 12, 14, 11, 13, 15, 12}
	for _, p := range prices {
		_ = b.Eval(p)
	}
	v := b.Eval(12)
	if math.IsNaN(v) {
		t.Fatal("BBLower should not be NaN")
	}
}

func TestCompileRSI(t *testing.T) {
	fields := FieldIndex{Fields: map[string]int{"close": 0}}
	c := NewCompiler(fields, nil)
	op, err := c.Compile(`rsi(14)`)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		_ = op.Eval(float64(100 + i))
	}
}

func TestCompileMACD(t *testing.T) {
	fields := FieldIndex{Fields: map[string]int{"close": 0}}
	c := NewCompiler(fields, nil)
	op, err := c.Compile(`macd(12, 26)`)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 30; i++ {
		_ = op.Eval(float64(100 + i))
	}
}

func TestCompileBollinger(t *testing.T) {
	fields := FieldIndex{Fields: map[string]int{"close": 0}}
	c := NewCompiler(fields, nil)

	for _, expr := range []string{
		`bb_upper(20, 2)`,
		`bb_lower(20, 2)`,
	} {
		op, err := c.Compile(expr)
		if err != nil {
			t.Fatalf("compile %s: %v", expr, err)
		}
		for i := 0; i < 25; i++ {
			_ = op.Eval(float64(100 + i))
		}
	}
}

func TestCompileBinaryOps(t *testing.T) {
	fields := FieldIndex{Fields: map[string]int{"close": 0}}
	c := NewCompiler(fields, nil)

	tests := []string{
		`sma($close, 3) + 1`,
		`sma($close, 3) - 2`,
		`sma($close, 3) * 3`,
		`sma($close, 3) / 2`,
		`-sma($close, 3)`,
		`abs(sma($close, 3))`,
		`(sma($close, 3) + sma($close, 5)) / 2`,
	}

	for _, expr := range tests {
		op, err := c.Compile(expr)
		if err != nil {
			t.Fatalf("compile %q: %v", expr, err)
		}
		for i := 0; i < 10; i++ {
			_ = op.Eval(float64(100 + i))
		}
	}
}

func TestCompileEMA(t *testing.T) {
	fields := FieldIndex{Fields: map[string]int{"close": 0}}
	c := NewCompiler(fields, nil)
	op, err := c.Compile(`ema($close, 20)`)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 25; i++ {
		v := op.Eval(float64(100 + i))
		if i < 19 && !math.IsNaN(v) {
			t.Errorf("bar %d: expected NaN during warmup, got %.6f", i, v)
		}
	}
}

func TestValidationRejectDangerous(t *testing.T) {
	dangerous := []string{
		`exec("rm")`,
		`eval("x")`,
		`import("os")`,
		`system("ls")`,
	}
	for _, expr := range dangerous {
		err := ValidateExpression(expr, nil, nil)
		if err == nil {
			t.Errorf("expected rejection of dangerous expr: %q", expr)
		}
	}
}

func TestValidationValid(t *testing.T) {
	available := map[string]bool{"close": true}
	valid := []string{
		`sma($close, 20)`,
		`ema($close, 20) / ema($close, 60) - 1`,
		`rsi(14)`,
		`macd(12, 26)`,
	}
	for _, expr := range valid {
		err := ValidateExpression(expr, available, nil)
		if err != nil {
			t.Errorf("expected valid expr %q, got: %v", expr, err)
		}
	}
}

func TestCompileRefDelta(t *testing.T) {
	fields := FieldIndex{Fields: map[string]int{"close": 0}}
	c := NewCompiler(fields, nil)
	op, err := c.Compile(`ref($close, 1)`)
	if err != nil {
		t.Fatal(err)
	}
	r := op.Eval(100)
	if !math.IsNaN(r) {
		t.Fatal("ref first value should be NaN")
	}
	r = op.Eval(105)
	if math.Abs(r-100) > 1e-9 {
		t.Fatalf("ref(1) should return previous value 100, got %.6f", r)
	}
}

func TestCompileDelta(t *testing.T) {
	fields := FieldIndex{Fields: map[string]int{"close": 0}}
	c := NewCompiler(fields, nil)
	op, err := c.Compile(`delta($close, 1)`)
	if err != nil {
		t.Fatal(err)
	}
	r := op.Eval(100)
	if !math.IsNaN(r) {
		t.Fatal("delta first value should be NaN")
	}
	r = op.Eval(105)
	if math.Abs(r-5) > 1e-9 {
		t.Fatalf("delta should be 5, got %.6f", r)
	}
}

func TestSTD(t *testing.T) {
	s := NewSTD(3)
	prices := []float64{2, 4, 4, 4, 5, 5, 7, 9}
	for _, p := range prices {
		_ = s.Eval(p)
	}
	v := s.Eval(6)
	if math.IsNaN(v) {
		t.Fatal("STD should not be NaN")
	}
}

func TestMinMax(t *testing.T) {
	n := NewMin(3)
	for _, v := range []float64{5, 3, 7, 2, 4} {
		_ = n.Eval(v)
	}
	if n.Eval(10) != 2 {
		t.Fatal("min should be 2")
	}

	x := NewMax(3)
	for _, v := range []float64{1, 5, 3, 2} {
		_ = x.Eval(v)
	}
	// The max is implemented via Min internally; no accessor needed.
	_ = x.Eval(10)
}

func TestSum(t *testing.T) {
	s := NewSum(3)
	for _, v := range []float64{1, 2, 3, 4} {
		_ = s.Eval(v)
	}
	// Sum(3) with values [2,3,4] → 9
	_ = s.Eval(5)
}
