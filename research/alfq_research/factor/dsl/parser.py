"""ALFQ factor DSL — recursive-descent parser (EBNF grammar)."""

from .ast_ import *
from .lexer import Lexer, Token, TokenKind


class Parser:
    def __init__(self, tokens: list[Token]):
        self.tokens = tokens
        self.pos = 0

    def _peek(self) -> Token:
        if self.pos >= len(self.tokens):
            return Token(TokenKind.EOF)
        return self.tokens[self.pos]

    def _next(self) -> Token:
        t = self._peek()
        if t.kind != TokenKind.EOF:
            self.pos += 1
        return t

    def _expect(self, k: TokenKind) -> Token:
        t = self._peek()
        if t.kind != k:
            raise SyntaxError(f"expected {k.name}, got {t.kind.name} at {t.pos}")
        return self._next()

    def parse(self) -> Node:
        node = self._expr()
        if self._peek().kind != TokenKind.EOF:
            raise SyntaxError(f"unexpected token after expression: {self._peek()}")
        return node

    def _expr(self) -> Node: return self._ternary()

    def _ternary(self) -> Node:
        cond = self._logic_or()
        if self._peek().kind == TokenKind.QUESTION:
            self._next()
            t = self._expr()
            self._expect(TokenKind.COLON)
            f = self._expr()
            return TernaryExpr(cond=cond, true_expr=t, false_expr=f)
        return cond

    def _logic_or(self) -> Node:
        left = self._logic_and()
        while self._peek().kind == TokenKind.OR:
            op = self._next().value
            left = BinaryExpr(op=op, left=left, right=self._logic_and())
        return left

    def _logic_and(self) -> Node:
        left = self._equality()
        while self._peek().kind == TokenKind.AND:
            op = self._next().value
            left = BinaryExpr(op=op, left=left, right=self._equality())
        return left

    def _equality(self) -> Node:
        left = self._compare()
        while self._peek().kind in (TokenKind.EQ, TokenKind.NE):
            op = self._next().value
            left = BinaryExpr(op=op, left=left, right=self._compare())
        return left

    def _compare(self) -> Node:
        left = self._addsub()
        if self._peek().kind in (TokenKind.LT, TokenKind.LE, TokenKind.GT, TokenKind.GE):
            op = self._next().value
            return BinaryExpr(op=op, left=left, right=self._addsub())
        return left

    def _addsub(self) -> Node:
        left = self._muldiv()
        while self._peek().kind in (TokenKind.PLUS, TokenKind.MINUS):
            op = self._next().value
            left = BinaryExpr(op=op, left=left, right=self._muldiv())
        return left

    def _muldiv(self) -> Node:
        left = self._unary()
        while self._peek().kind in (TokenKind.STAR, TokenKind.SLASH, TokenKind.PERCENT):
            op = self._next().value
            left = BinaryExpr(op=op, left=left, right=self._unary())
        return left

    def _unary(self) -> Node:
        if self._peek().kind in (TokenKind.MINUS, TokenKind.NOT):
            op = self._next().value
            return UnaryExpr(op=op, expr=self._primary())
        return self._primary()

    def _primary(self) -> Node:
        tok = self._peek()
        if tok.kind == TokenKind.NUMBER:
            self._next()
            return NumberLit(value=float(tok.value))
        if tok.kind == TokenKind.BOOL:
            self._next()
            return BoolLit(value=tok.value == "true")
        if tok.kind == TokenKind.STRING:
            self._next()
            return StringLit(value=tok.value)
        if tok.kind == TokenKind.FIELD:
            self._next()
            return FieldRef(name=tok.value)
        if tok.kind == TokenKind.LPAREN:
            self._next()
            expr = self._expr()
            self._expect(TokenKind.RPAREN)
            return expr
        if tok.kind == TokenKind.IDENT:
            self._next()
            if self._peek().kind == TokenKind.LPAREN:
                return self._call_args(tok.value)
            return FactorRef(name=tok.value)
        raise SyntaxError(f"unexpected token {tok.kind.name} at {tok.pos}")

    def _call_args(self, name: str) -> CallExpr:
        self._next()  # consume '('
        args = []
        if self._peek().kind != TokenKind.RPAREN:
            while True:
                args.append(self._expr())
                if self._peek().kind != TokenKind.COMMA:
                    break
                self._next()
        self._expect(TokenKind.RPAREN)
        return CallExpr(name=name, args=args)


def parse(src: str) -> Node:
    toks = Lexer(src).lex_all()
    return Parser(toks).parse()
