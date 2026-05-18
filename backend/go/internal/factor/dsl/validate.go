package dsl

import (
	"fmt"
	"strings"
	"time"
)

// ValidateExpression validates a DSL expression string for syntax, safety, and reference integrity.
// It returns an error on failure, nil on success.
func ValidateExpression(expr string, availableFields, availableFactors map[string]bool) error {
	// Safety checks
	if len(expr) > 4096 {
		return fmt.Errorf("validation: expression exceeds 4 KB limit")
	}

	// Parse with timeout
	type result struct {
		node Node
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		node, err := Parse(expr)
		ch <- result{node, err}
	}()

	select {
	case r := <-ch:
		if r.err != nil {
			return fmt.Errorf("validation: parse error: %w", r.err)
		}
		return validateNode(r.node, availableFields, availableFactors)
	case <-time.After(100 * time.Millisecond):
		return fmt.Errorf("validation: parse timeout (100 ms)")
	}
}

func validateNode(node Node, fields, factors map[string]bool) error {
	switch n := node.(type) {
	case *FieldRef:
		if fields != nil && !fields[n.Name] {
			return fmt.Errorf("validation: unknown field $%s", n.Name)
		}
	case *CallExpr:
		if n.Name == "" {
			return fmt.Errorf("validation: empty function name")
		}
		// Security: block dangerous tokens
		dangerous := []string{"exec", "eval", "import", "open", "os", "sys", "subprocess", "__"}
		for _, d := range dangerous {
			if strings.Contains(strings.ToLower(n.Name), d) || strings.Contains(strings.ToLower(exprToString(node)), d) {
				return fmt.Errorf("validation: dangerous token %q rejected", d)
			}
		}
		for _, arg := range n.Args {
			if err := validateNode(arg, fields, factors); err != nil {
				return err
			}
		}
	case *FactorRef:
		if factors != nil && !factors[n.Name] {
			return fmt.Errorf("validation: unknown factor reference %q", n.Name)
		}
	case *BinaryExpr:
		if err := validateNode(n.Left, fields, factors); err != nil {
			return err
		}
		return validateNode(n.Right, fields, factors)
	case *UnaryExpr:
		return validateNode(n.Expr, fields, factors)
	case *TernaryExpr:
		if err := validateNode(n.Cond, fields, factors); err != nil {
			return err
		}
		if err := validateNode(n.True, fields, factors); err != nil {
			return err
		}
		return validateNode(n.False, fields, factors)
	}
	return nil
}

func exprToString(n Node) string {
	switch x := n.(type) {
	case *NumberLit:
		return fmt.Sprintf("%v", x.Value)
	case *StringLit:
		return fmt.Sprintf("%q", x.Value)
	case *FieldRef:
		return "$" + x.Name
	case *CallExpr:
		return x.Name
	case *FactorRef:
		return x.Name
	}
	return ""
}
