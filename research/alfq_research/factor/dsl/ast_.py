"""ALFQ factor DSL — AST node types."""

from dataclasses import dataclass, field


@dataclass
class NumberLit:
    value: float

@dataclass
class BoolLit:
    value: bool

@dataclass
class StringLit:
    value: str

@dataclass
class FieldRef:
    name: str

@dataclass
class FactorRef:
    name: str

@dataclass
class UnaryExpr:
    op: str
    expr: "Node"

@dataclass
class BinaryExpr:
    op: str
    left: "Node"
    right: "Node"

@dataclass
class TernaryExpr:
    cond: "Node"
    true_expr: "Node"
    false_expr: "Node"

@dataclass
class CallExpr:
    name: str
    args: list["Node"] = field(default_factory=list)


Node = NumberLit | BoolLit | StringLit | FieldRef | FactorRef | UnaryExpr | BinaryExpr | TernaryExpr | CallExpr
