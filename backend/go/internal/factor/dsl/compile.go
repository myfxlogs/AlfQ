package dsl

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Compiler compiles a DSL expression AST into an evaluable Op.
type Compiler struct {
	fields    map[string]int // field name → FieldIndex
	factors   map[string]Op
	validated bool
}

// FieldIndex maps a field name to an index in the []float64 field array.
type FieldIndex struct {
	Fields map[string]int
}

// NewCompiler creates a Compiler with the given field mapping.
func NewCompiler(fields FieldIndex, factors map[string]Op) *Compiler {
	return &Compiler{fields: fields.Fields, factors: factors}
}

// Compile parses and compiles an expression into an evaluable Op.
func (c *Compiler) Compile(expr string) (Op, error) {
	node, err := Parse(expr)
	if err != nil {
		return nil, fmt.Errorf("compile: %w", err)
	}
	return c.compileNode(node)
}

func (c *Compiler) compileNode(node Node) (Op, error) {
	switch n := node.(type) {
	case *NumberLit:
		return &constOp{val: n.Value}, nil
	case *FieldRef:
		idx, ok := c.fields[n.Name]
		if !ok {
			return nil, fmt.Errorf("compile: unknown field $%s", n.Name)
		}
		return &fieldOp{idx: idx}, nil
	case *FactorRef:
		op, ok := c.factors[n.Name]
		if !ok {
			return nil, fmt.Errorf("compile: unknown factor %q", n.Name)
		}
		return op, nil
	case *BinaryExpr:
		return c.compileBinary(n)
	case *UnaryExpr:
		return c.compileUnary(n)
	case *TernaryExpr:
		return c.compileTernary(n)
	case *CallExpr:
		return c.compileCall(n)
	default:
		return nil, fmt.Errorf("compile: unsupported node type %T", n)
	}
}

func (c *Compiler) compileBinary(n *BinaryExpr) (Op, error) {
	left, err := c.compileNode(n.Left)
	if err != nil {
		return nil, err
	}
	right, err := c.compileNode(n.Right)
	if err != nil {
		return nil, err
	}
	return &binaryOp{op: n.Op, left: left, right: right}, nil
}

func (c *Compiler) compileUnary(n *UnaryExpr) (Op, error) {
	inner, err := c.compileNode(n.Expr)
	if err != nil {
		return nil, err
	}
	return &unaryOp{op: n.Op, inner: inner}, nil
}

func (c *Compiler) compileTernary(n *TernaryExpr) (Op, error) {
	cond, err := c.compileNode(n.Cond)
	if err != nil {
		return nil, err
	}
	t, err := c.compileNode(n.True)
	if err != nil {
		return nil, err
	}
	f, err := c.compileNode(n.False)
	if err != nil {
		return nil, err
	}
	return &ternaryOp{cond: cond, t: t, f: f}, nil
}

func (c *Compiler) compileCall(n *CallExpr) (Op, error) {
	switch strings.ToLower(n.Name) {
	case "sma":
		return c.newWindowOp(n.Args, func(n int) Op { return NewSMA(n) })
	case "ema":
		return c.newWindowOp(n.Args, func(n int) Op { return NewEMA(n) })
	case "wma":
		return c.newWindowOp(n.Args, func(n int) Op { return NewWMA(n) })
	case "std":
		return c.newWindowOp(n.Args, func(n int) Op { return NewSTD(n) })
	case "var":
		return c.newWindowOp(n.Args, func(n int) Op { return NewVAR(n) })
	case "min":
		return c.newWindowOp(n.Args, func(n int) Op { return NewMin(n) })
	case "max":
		return c.newWindowOp(n.Args, func(n int) Op { return NewMax(n) })
	case "sum":
		return c.newWindowOp(n.Args, func(n int) Op { return NewSum(n) })
	case "ref":
		return c.newWindowOp(n.Args, func(n int) Op { return NewRef(n) })
	case "delta":
		return c.newWindowOp(n.Args, func(n int) Op { return NewDelta(n) })
	case "pct_change":
		return c.newWindowOp(n.Args, func(n int) Op { return NewPctChange(n) })
	case "zscore":
		return c.newWindowOp(n.Args, func(n int) Op { return NewZScore(n) })
	case "rank":
		return c.newWindowOp(n.Args, func(n int) Op { return NewRank(n) })
	case "rsi":
		return c.newWindowOp(n.Args, func(n int) Op { return NewRSI(n) })
	case "atr":
		return c.newWindowOp(n.Args, func(n int) Op { return NewATR(n) })
	case "abs", "sign", "log", "exp", "sqrt":
		inner, err := c.compileNode(n.Args[0])
		if err != nil {
			return nil, err
		}
		return &scalarOp{name: n.Name, inner: inner}, nil
	case "pow":
		inner, err := c.compileNode(n.Args[0])
		if err != nil {
			return nil, err
		}
		expv, ok := n.Args[1].(*NumberLit)
		if !ok {
			return nil, fmt.Errorf("compile: pow second arg must be number literal")
		}
		return &powOp{inner: inner, exp: expv.Value}, nil
	case "if_":
		if len(n.Args) != 3 {
			return nil, fmt.Errorf("compile: if_ requires 3 args")
		}
		cond, err := c.compileNode(n.Args[0])
		if err != nil {
			return nil, err
		}
		a, err := c.compileNode(n.Args[1])
		if err != nil {
			return nil, err
		}
		b, err := c.compileNode(n.Args[2])
		if err != nil {
			return nil, err
		}
		return &ternaryOp{cond: cond, t: a, f: b}, nil
	case "macd":
		if len(n.Args) < 2 {
			return nil, fmt.Errorf("compile: macd requires at least 2 args (fast, slow)")
		}
		fast := argInt(n.Args, 0, 12)
		slow := argInt(n.Args, 1, 26)
		inner, err := c.compileNode(n.Args[0])
		if err != nil {
			return nil, err
		}
		macdOp := NewMACD(fast, slow)
		return &wrapOp{inner: inner, outer: macdOp}, nil
	case "bb_upper", "bb_lower":
		if len(n.Args) < 2 {
			return nil, fmt.Errorf("compile: %s requires at least 2 args", n.Name)
		}
		period := argInt(n.Args, 0, 20)
		k := argFloat(n.Args, 1, 2.0)
		inner, err := c.compileNode(n.Args[0])
		if err != nil {
			return nil, err
		}
		var outer Op
		if n.Name == "bb_upper" {
			outer = NewBBUpper(period, k)
		} else {
			outer = NewBBLower(period, k)
		}
		return &wrapOp{inner: inner, outer: outer}, nil
	default:
		return nil, fmt.Errorf("compile: unknown function %q", n.Name)
	}
}

// Compiled operator wrappers

type constOp struct{ val float64 }

func (c *constOp) Eval(v float64) float64 { return c.val }
func (c *constOp) Reset()                 {}
func (c *constOp) Warmup() int            { return 0 }

type fieldOp struct{ idx int }

func (f *fieldOp) Eval(v float64) float64 { return v }
func (f *fieldOp) Reset()                 {}
func (f *fieldOp) Warmup() int            { return 0 }

type binaryOp struct {
	op          string
	left, right Op
}

func (b *binaryOp) Eval(v float64) float64 {
	l := b.left.Eval(v)
	r := b.right.Eval(v)
	switch b.op {
	case "+":
		return l + r
	case "-":
		return l - r
	case "*":
		return l * r
	case "/":
		if r == 0 {
			return math.NaN()
		}
		return l / r
	case "%":
		return math.Mod(l, r)
	case "==":
		if l == r {
			return 1
		}
		return 0
	case "!=":
		if l != r {
			return 1
		}
		return 0
	case "<":
		if l < r {
			return 1
		}
		return 0
	case "<=":
		if l <= r {
			return 1
		}
		return 0
	case ">":
		if l > r {
			return 1
		}
		return 0
	case ">=":
		if l >= r {
			return 1
		}
		return 0
	case "&&":
		if l != 0 && r != 0 {
			return 1
		}
		return 0
	case "||":
		if l != 0 || r != 0 {
			return 1
		}
		return 0
	}
	return math.NaN()
}
func (b *binaryOp) Reset() {
	b.left.Reset()
	b.right.Reset()
}
func (b *binaryOp) Warmup() int {
	return nMax(b.left.Warmup(), b.right.Warmup())
}

type unaryOp struct {
	op    string
	inner Op
}

func (u *unaryOp) Eval(v float64) float64 {
	x := u.inner.Eval(v)
	switch u.op {
	case "-":
		return -x
	case "!":
		if x == 0 {
			return 1
		}
		return 0
	}
	return math.NaN()
}
func (u *unaryOp) Reset()      { u.inner.Reset() }
func (u *unaryOp) Warmup() int { return u.inner.Warmup() }

type ternaryOp struct {
	cond, t, f Op
}

func (t *ternaryOp) Eval(v float64) float64 {
	if t.cond.Eval(v) != 0 {
		return t.t.Eval(v)
	}
	return t.f.Eval(v)
}
func (t *ternaryOp) Reset() { t.cond.Reset(); t.t.Reset(); t.f.Reset() }
func (t *ternaryOp) Warmup() int {
	return nMax(t.cond.Warmup(), nMax(t.t.Warmup(), t.f.Warmup()))
}

type scalarOp struct {
	name  string
	inner Op
}

func (s *scalarOp) Eval(v float64) float64 {
	x := s.inner.Eval(v)
	switch s.name {
	case "abs":
		return Abs(x)
	case "sign":
		return Sign(x)
	case "log":
		return Log(x)
	case "exp":
		return Exp(x)
	case "sqrt":
		return Sqrt(x)
	}
	return math.NaN()
}
func (s *scalarOp) Reset()      { s.inner.Reset() }
func (s *scalarOp) Warmup() int { return s.inner.Warmup() }

type powOp struct {
	inner Op
	exp   float64
}

func (p *powOp) Eval(v float64) float64 { return Pow(p.inner.Eval(v), p.exp) }
func (p *powOp) Reset()                 { p.inner.Reset() }
func (p *powOp) Warmup() int            { return p.inner.Warmup() }

type wrapOp struct {
	inner, outer Op
}

func (w *wrapOp) Eval(v float64) float64 {
	x := w.inner.Eval(v)
	if math.IsNaN(x) {
		return math.NaN()
	}
	return w.outer.Eval(x)
}
func (w *wrapOp) Reset()      { w.inner.Reset(); w.outer.Reset() }
func (w *wrapOp) Warmup() int { return nMax(w.inner.Warmup(), w.outer.Warmup()) }

func (c *Compiler) newWindowOp(args []Node, factory func(int) Op) (Op, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("compile: window op requires at least 1 arg")
	}
	n := argInt(args, 0, 14)
	// First arg might be the field, second is the window. For single-arg window ops (rsi, atr).
	inner, err := c.compileNode(args[0])
	if err != nil {
		return nil, err
	}
	// Check if inner is a fieldOp — if so, it's a simple window op like sma($close, 20)
	outer := factory(n)
	return &wrapOp{inner: inner, outer: outer}, nil
}

func argInt(args []Node, idx, defaultVal int) int {
	if idx+1 >= len(args) {
		return defaultVal
	}
	if lit, ok := args[idx+1].(*NumberLit); ok {
		return int(lit.Value)
	}
	return defaultVal
}

func argFloat(args []Node, idx int, defaultVal float64) float64 {
	if idx+1 >= len(args) {
		return defaultVal
	}
	if lit, ok := args[idx+1].(*NumberLit); ok {
		return lit.Value
	}
	return defaultVal
}

func nMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Keep for compatibility
var _ = strconv.FormatFloat
