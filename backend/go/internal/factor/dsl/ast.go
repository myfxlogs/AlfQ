// Package dsl implements the ALFQ factor DSL engine (Go).
// Grammar: see docs/09-因子DSL规范.md §1.4 (EBNF).
package dsl

// Node is an AST node.
type Node interface {
	node()
}

// NumberLit: 42, 3.14, -1.5e-3
type NumberLit struct{ Value float64 }

// BoolLit: true, false
type BoolLit struct{ Value bool }

// StringLit: "EURUSD"
type StringLit struct{ Value string }

// FieldRef: $open, $close, $volume
type FieldRef struct{ Name string }

// UnaryExpr: -x, !x
type UnaryExpr struct {
	Op   string // "-" or "!"
	Expr Node
}

// BinaryExpr: a + b, a > b, a && b
type BinaryExpr struct {
	Op          string
	Left, Right Node
}

// TernaryExpr: cond ? a : b
type TernaryExpr struct {
	Cond, True, False Node
}

// CallExpr: func(args...)
type CallExpr struct {
	Name string
	Args []Node
}

// FactorRef: user-defined factor (no $ prefix)
type FactorRef struct{ Name string }

func (*NumberLit) node()   {}
func (*BoolLit) node()     {}
func (*StringLit) node()   {}
func (*FieldRef) node()     {}
func (*UnaryExpr) node()    {}
func (*BinaryExpr) node()   {}
func (*TernaryExpr) node()  {}
func (*CallExpr) node()     {}
func (*FactorRef) node()    {}
