"""ALFQ factor DSL — lexer (tokenizer)."""

from enum import Enum, auto
from dataclasses import dataclass


class TokenKind(Enum):
    EOF = auto()
    NUMBER = auto()
    BOOL = auto()
    STRING = auto()
    IDENT = auto()
    FIELD = auto()
    LPAREN = auto()
    RPAREN = auto()
    COMMA = auto()
    QUESTION = auto()
    COLON = auto()
    PLUS = auto()
    MINUS = auto()
    STAR = auto()
    SLASH = auto()
    PERCENT = auto()
    EQ = auto()
    NE = auto()
    LT = auto()
    LE = auto()
    GT = auto()
    GE = auto()
    AND = auto()
    OR = auto()
    NOT = auto()


@dataclass
class Token:
    kind: TokenKind
    value: str = ""
    pos: int = 0


class Lexer:
    def __init__(self, src: str):
        self.src = src
        self.pos = 0

    def lex_all(self) -> list[Token]:
        toks = []
        while True:
            tok = self._next()
            toks.append(tok)
            if tok.kind == TokenKind.EOF:
                break
        return toks

    def _next(self) -> Token:
        self._skip_space()
        if self.pos >= len(self.src):
            return Token(TokenKind.EOF, pos=self.pos)

        ch = self.src[self.pos]

        if ch.isdigit() or (ch == "." and self.pos + 1 < len(self.src) and self.src[self.pos + 1].isdigit()):
            return self._lex_number()
        if ch == "$":
            start = self.pos
            self.pos += 1
            name = self._lex_ident()
            return Token(TokenKind.FIELD, name, start)
        if ch.isalpha() or ch == "_":
            start = self.pos
            name = self._lex_ident()
            if name in ("true", "false"):
                return Token(TokenKind.BOOL, name, start)
            return Token(TokenKind.IDENT, name, start)
        if ch == '"':
            return self._lex_string()

        if self.pos + 1 < len(self.src):
            two = self.src[self.pos:self.pos + 2]
            two_map = {"==": TokenKind.EQ, "!=": TokenKind.NE, "<=": TokenKind.LE, ">=": TokenKind.GE, "&&": TokenKind.AND, "||": TokenKind.OR}
            if two in two_map:
                self.pos += 2
                return Token(two_map[two], value=two, pos=self.pos - 2)

        self.pos += 1
        single = {"(": TokenKind.LPAREN, ")": TokenKind.RPAREN, ",": TokenKind.COMMA, "?": TokenKind.QUESTION,
                   ":": TokenKind.COLON, "+": TokenKind.PLUS, "-": TokenKind.MINUS, "*": TokenKind.STAR,
                   "/": TokenKind.SLASH, "%": TokenKind.PERCENT, "<": TokenKind.LT, ">": TokenKind.GT, "!": TokenKind.NOT}
        if ch in single:
            return Token(single[ch], value=ch, pos=self.pos - 1)
        raise ValueError(f"lex: unexpected character {ch!r} at {self.pos - 1}")

    def _lex_number(self) -> Token:
        start = self.pos
        while self.pos < len(self.src) and (self.src[self.pos].isdigit() or self.src[self.pos] == "."):
            self.pos += 1
        if self.pos < len(self.src) and self.src[self.pos] in ("e", "E"):
            self.pos += 1
            if self.pos < len(self.src) and self.src[self.pos] in ("+", "-"):
                self.pos += 1
            while self.pos < len(self.src) and self.src[self.pos].isdigit():
                self.pos += 1
        return Token(TokenKind.NUMBER, self.src[start:self.pos], start)

    def _lex_string(self) -> Token:
        start = self.pos
        self.pos += 1
        while self.pos < len(self.src):
            if self.src[self.pos] == '"':
                self.pos += 1
                return Token(TokenKind.STRING, self.src[start + 1:self.pos - 1], start)
            self.pos += 1
        raise ValueError(f"lex: unterminated string at {start}")

    def _lex_ident(self) -> str:
        start = self.pos
        while self.pos < len(self.src) and (self.src[self.pos].isalnum() or self.src[self.pos] == "_"):
            self.pos += 1
        return self.src[start:self.pos]

    def _skip_space(self):
        while self.pos < len(self.src) and self.src[self.pos] in (" ", "\t", "\n"):
            self.pos += 1
