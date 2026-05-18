package dsl

import (
	"fmt"
	"strconv"
)

// Parser is a recursive-descent parser for the factor DSL.
// Grammar (EBNF): see docs/09-因子DSL规范.md §1.4.
type Parser struct {
	toks []Token
	pos  int
}

func NewParser(tokens []Token) *Parser {
	return &Parser{toks: tokens}
}

func Parse(src string) (Node, error) {
	lex := NewLexer(src)
	toks, err := lex.LexAll()
	if err != nil {
		return nil, err
	}
	p := NewParser(toks)
	node, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if p.peek().Kind != TokEOF {
		return nil, fmt.Errorf("parse: unexpected token %s after expression", p.peek())
	}
	return node, nil
}

func (p *Parser) peek() Token {
	if p.pos >= len(p.toks) {
		return Token{Kind: TokEOF}
	}
	return p.toks[p.pos]
}

func (p *Parser) next() Token {
	t := p.peek()
	if t.Kind != TokEOF {
		p.pos++
	}
	return t
}

func (p *Parser) expect(k TokenKind) (Token, error) {
	t := p.peek()
	if t.Kind != k {
		return t, fmt.Errorf("parse: expected %s, got %s at %d", tokenKindName(k), t, t.Pos)
	}
	return p.next(), nil
}

// expr = ternary
func (p *Parser) parseExpr() (Node, error) {
	return p.parseTernary()
}

// ternary = logic_or [ "?" expr ":" expr ]
func (p *Parser) parseTernary() (Node, error) {
	cond, err := p.parseLogicOr()
	if err != nil {
		return nil, err
	}
	if p.peek().Kind == TokQuestion {
		p.next() // consume ?
		trueExpr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokColon); err != nil {
			return nil, err
		}
		falseExpr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		return &TernaryExpr{Cond: cond, True: trueExpr, False: falseExpr}, nil
	}
	return cond, nil
}

// logic_or = logic_and { "||" logic_and }
func (p *Parser) parseLogicOr() (Node, error) {
	left, err := p.parseLogicAnd()
	if err != nil {
		return nil, err
	}
	for p.peek().Kind == TokOr {
		op := p.next().Value
		right, err := p.parseLogicAnd()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: op, Left: left, Right: right}
	}
	return left, nil
}

// logic_and = equality { "&&" equality }
func (p *Parser) parseLogicAnd() (Node, error) {
	left, err := p.parseEquality()
	if err != nil {
		return nil, err
	}
	for p.peek().Kind == TokAnd {
		op := p.next().Value
		right, err := p.parseEquality()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: op, Left: left, Right: right}
	}
	return left, nil
}

// equality = compare { ("==" | "!=") compare }
func (p *Parser) parseEquality() (Node, error) {
	left, err := p.parseCompare()
	if err != nil {
		return nil, err
	}
	for p.peek().Kind == TokEq || p.peek().Kind == TokNe {
		op := p.next().Value
		right, err := p.parseCompare()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: op, Left: left, Right: right}
	}
	return left, nil
}

// compare = addsub { ("<" | "<=" | ">" | ">=") addsub }
func (p *Parser) parseCompare() (Node, error) {
	left, err := p.parseAddSub()
	if err != nil {
		return nil, err
	}
	switch p.peek().Kind {
	case TokLt, TokLe, TokGt, TokGe:
		op := p.next().Value
		right, err := p.parseAddSub()
		if err != nil {
			return nil, err
		}
		return &BinaryExpr{Op: op, Left: left, Right: right}, nil
	}
	return left, nil
}

// addsub = muldiv { ("+" | "-") muldiv }
func (p *Parser) parseAddSub() (Node, error) {
	left, err := p.parseMulDiv()
	if err != nil {
		return nil, err
	}
	for p.peek().Kind == TokPlus || p.peek().Kind == TokMinus {
		op := p.next().Value
		right, err := p.parseMulDiv()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: op, Left: left, Right: right}
	}
	return left, nil
}

// muldiv = unary { ("*" | "/" | "%") unary }
func (p *Parser) parseMulDiv() (Node, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.peek().Kind == TokStar || p.peek().Kind == TokSlash || p.peek().Kind == TokPercent {
		op := p.next().Value
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: op, Left: left, Right: right}
	}
	return left, nil
}

// unary = [ "-" | "!" ] primary
func (p *Parser) parseUnary() (Node, error) {
	if p.peek().Kind == TokMinus || p.peek().Kind == TokNot {
		op := p.next().Value
		expr, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Op: op, Expr: expr}, nil
	}
	return p.parsePrimary()
}

// primary = number | string | bool | field | call | "(" expr ")" | ident (factor ref or call)
func (p *Parser) parsePrimary() (Node, error) {
	tok := p.peek()
	switch tok.Kind {
	case TokNumber:
		p.next()
		v, _ := strconv.ParseFloat(tok.Value, 64)
		return &NumberLit{Value: v}, nil
	case TokBool:
		p.next()
		return &BoolLit{Value: tok.Value == "true"}, nil
	case TokString:
		p.next()
		return &StringLit{Value: tok.Value}, nil
	case TokField:
		p.next()
		return &FieldRef{Name: tok.Value}, nil
	case TokLParen:
		p.next()
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokRParen); err != nil {
			return nil, err
		}
		return expr, nil
	case TokIdent:
		p.next()
		// Check if followed by '(' → function call
		if p.peek().Kind == TokLParen {
			return p.parseCallArgs(tok.Value)
		}
		// Otherwise, factor reference
		return &FactorRef{Name: tok.Value}, nil
	default:
		return nil, fmt.Errorf("parse: unexpected token %s at %d", tok, tok.Pos)
	}
}

func (p *Parser) parseCallArgs(name string) (Node, error) {
	p.next() // consume '('
	var args []Node
	if p.peek().Kind != TokRParen {
		for {
			arg, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
			if p.peek().Kind != TokComma {
				break
			}
			p.next()
		}
	}
	if _, err := p.expect(TokRParen); err != nil {
		return nil, err
	}
	return &CallExpr{Name: name, Args: args}, nil
}
