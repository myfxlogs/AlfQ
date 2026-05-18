package dsl

import "fmt"

// Token types.
type TokenKind int

const (
	TokEOF TokenKind = iota
	TokNumber
	TokBool
	TokString
	TokIdent
	TokField // $xxx
	TokLParen
	TokRParen
	TokComma
	TokQuestion
	TokColon
	TokPlus
	TokMinus
	TokStar
	TokSlash
	TokPercent
	TokEq
	TokNe
	TokLt
	TokLe
	TokGt
	TokGe
	TokAnd
	TokOr
	TokNot
)

type Token struct {
	Kind  TokenKind
	Value string
	Pos   int // byte offset in source
}

func (t Token) String() string {
	switch t.Kind {
	case TokEOF:
		return "EOF"
	case TokNumber, TokBool, TokString, TokIdent, TokField:
		return fmt.Sprintf("%s(%s)", tokenKindName(t.Kind), t.Value)
	default:
		return tokenKindName(t.Kind)
	}
}

func tokenKindName(k TokenKind) string {
	names := map[TokenKind]string{
		TokEOF: "EOF", TokNumber: "NUM", TokBool: "BOOL", TokString: "STR",
		TokIdent: "IDENT", TokField: "FIELD", TokLParen: "(", TokRParen: ")",
		TokComma: ",", TokQuestion: "?", TokColon: ":",
		TokPlus: "+", TokMinus: "-", TokStar: "*", TokSlash: "/", TokPercent: "%",
		TokEq: "==", TokNe: "!=", TokLt: "<", TokLe: "<=", TokGt: ">", TokGe: ">=",
		TokAnd: "&&", TokOr: "||", TokNot: "!",
	}
	if n, ok := names[k]; ok {
		return n
	}
	return "?"
}

// Lexer tokenizes a DSL expression string.
type Lexer struct {
	src string
	pos int
}

func NewLexer(src string) *Lexer {
	return &Lexer{src: src}
}

func (l *Lexer) LexAll() ([]Token, error) {
	var toks []Token
	for {
		tok, err := l.Next()
		if err != nil {
			return nil, err
		}
		toks = append(toks, tok)
		if tok.Kind == TokEOF {
			break
		}
	}
	return toks, nil
}

func (l *Lexer) Next() (Token, error) {
	l.skipSpace()
	if l.pos >= len(l.src) {
		return Token{Kind: TokEOF, Pos: l.pos}, nil
	}

	ch := l.src[l.pos]

	// Numbers
	if isDigit(ch) || (ch == '.' && l.pos+1 < len(l.src) && isDigit(l.src[l.pos+1])) {
		return l.lexNumber()
	}

	// Field: $xxx
	if ch == '$' {
		start := l.pos
		l.pos++
		if l.pos >= len(l.src) || !isIdentStart(l.src[l.pos]) {
			return Token{}, fmt.Errorf("lex: expected identifier after $ at %d", start)
		}
		name := l.lexIdentRest()
		return Token{Kind: TokField, Value: name, Pos: start}, nil
	}

	// Identifiers and keywords
	if isIdentStart(ch) {
		start := l.pos
		name := l.lexIdentRest()
		switch name {
		case "true":
			return Token{Kind: TokBool, Value: "true", Pos: start}, nil
		case "false":
			return Token{Kind: TokBool, Value: "false", Pos: start}, nil
		default:
			return Token{Kind: TokIdent, Value: name, Pos: start}, nil
		}
	}

	// Strings
	if ch == '"' {
		return l.lexString()
	}

	// Two-char operators
	if l.pos+1 < len(l.src) {
		two := l.src[l.pos : l.pos+2]
		switch two {
		case "==", "!=", "<=", ">=", "&&", "||":
			l.pos += 2
			return Token{Kind: l.twoCharKind(two), Pos: l.pos - 2}, nil
		}
	}

	// Single-char tokens
	l.pos++
	switch ch {
	case '(':
		return Token{Kind: TokLParen, Pos: l.pos - 1}, nil
	case ')':
		return Token{Kind: TokRParen, Pos: l.pos - 1}, nil
	case ',':
		return Token{Kind: TokComma, Pos: l.pos - 1}, nil
	case '?':
		return Token{Kind: TokQuestion, Pos: l.pos - 1}, nil
	case ':':
		return Token{Kind: TokColon, Pos: l.pos - 1}, nil
	case '+':
		return Token{Kind: TokPlus, Pos: l.pos - 1}, nil
	case '-':
		return Token{Kind: TokMinus, Pos: l.pos - 1}, nil
	case '*':
		return Token{Kind: TokStar, Pos: l.pos - 1}, nil
	case '/':
		return Token{Kind: TokSlash, Pos: l.pos - 1}, nil
	case '%':
		return Token{Kind: TokPercent, Pos: l.pos - 1}, nil
	case '<':
		return Token{Kind: TokLt, Pos: l.pos - 1}, nil
	case '>':
		return Token{Kind: TokGt, Pos: l.pos - 1}, nil
	case '!':
		return Token{Kind: TokNot, Pos: l.pos - 1}, nil
	default:
		return Token{}, fmt.Errorf("lex: unexpected character %q at %d", ch, l.pos-1)
	}
}

func (l *Lexer) lexNumber() (Token, error) {
	start := l.pos
	for l.pos < len(l.src) && (isDigit(l.src[l.pos]) || l.src[l.pos] == '.') {
		l.pos++
	}
	// Optional exponent
	if l.pos < len(l.src) && (l.src[l.pos] == 'e' || l.src[l.pos] == 'E') {
		l.pos++
		if l.pos < len(l.src) && (l.src[l.pos] == '+' || l.src[l.pos] == '-') {
			l.pos++
		}
		for l.pos < len(l.src) && isDigit(l.src[l.pos]) {
			l.pos++
		}
	}
	return Token{Kind: TokNumber, Value: l.src[start:l.pos], Pos: start}, nil
}

func (l *Lexer) lexString() (Token, error) {
	start := l.pos
	l.pos++ // skip opening "
	for l.pos < len(l.src) {
		if l.src[l.pos] == '"' {
			l.pos++ // skip closing "
			return Token{Kind: TokString, Value: l.src[start+1 : l.pos-1], Pos: start}, nil
		}
		l.pos++
	}
	return Token{}, fmt.Errorf("lex: unterminated string at %d", start)
}

func (l *Lexer) lexIdentRest() string {
	start := l.pos
	for l.pos < len(l.src) && isIdentPart(l.src[l.pos]) {
		l.pos++
	}
	return l.src[start:l.pos]
}

func (l *Lexer) skipSpace() {
	for l.pos < len(l.src) && (l.src[l.pos] == ' ' || l.src[l.pos] == '\t' || l.src[l.pos] == '\n') {
		l.pos++
	}
}

func (l *Lexer) twoCharKind(s string) TokenKind {
	switch s {
	case "==":
		return TokEq
	case "!=":
		return TokNe
	case "<=":
		return TokLe
	case ">=":
		return TokGe
	case "&&":
		return TokAnd
	case "||":
		return TokOr
	}
	return TokEOF
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
func isIdentStart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}
func isIdentPart(c byte) bool {
	return isIdentStart(c) || isDigit(c)
}
