"""ALFQ factor DSL — streaming compiler and operators.

Maps DSL AST → evaluable operator tree that consumes bar-by-bar float values.
All 22+ operators from docs/09 with identical semantics to the Go engine.
"""

from __future__ import annotations

import math
from abc import ABC, abstractmethod
from .ast_ import *


# ═══════════════════════════════════════════════════════════════════════
# Operator interface
# ═══════════════════════════════════════════════════════════════════════

class Op(ABC):
    @abstractmethod
    def eval(self, v: float) -> float: ...
    def reset(self): pass
    def warmup(self) -> int: return 0


class DualOp(ABC):
    """Operator that consumes two input series per bar."""
    @abstractmethod
    def eval(self, x: float, y: float) -> float: ...
    def reset(self): pass
    def warmup(self) -> int: return 0


# ═══════════════════════════════════════════════════════════════════════
# Window / moving-average operators
# ═══════════════════════════════════════════════════════════════════════

class SMA(Op):
    def __init__(self, n: int):
        self.n = n
        self.buf = [0.0] * n
        self.idx = 0
        self.sum = 0.0
        self.count = 0
    def warmup(self): return self.n
    def eval(self, v):
        old = self.buf[self.idx]
        self.buf[self.idx] = v
        self.idx = (self.idx + 1) % self.n
        self.sum += v - old
        if self.count < self.n: self.count += 1
        if self.count < self.n: return math.nan
        return self.sum / self.n
    def reset(self): self.__init__(self.n)


class EMA(Op):
    def __init__(self, n: int):
        self.n = n
        self.alpha = 2.0 / (n + 1)
        self.value = 0.0
        self.count = 0
    def warmup(self): return self.n
    def eval(self, v):
        if self.count == 0: self.value = v
        else: self.value = self.alpha * v + (1 - self.alpha) * self.value
        self.count += 1
        if self.count < self.n: return math.nan
        return self.value
    def reset(self): self.__init__(self.n)


class WMA(Op):
    """Weighted Moving Average — linear weights 1..n (matching Go)."""
    def __init__(self, n: int):
        self.n = n
        self.buf = [0.0] * n
        self.idx = 0
        self.count = 0
    def warmup(self): return self.n
    def eval(self, v):
        self.buf[self.idx] = v
        self.idx = (self.idx + 1) % self.n
        if self.count < self.n: self.count += 1
        if self.count < self.n: return math.nan
        total = 0.0
        wsum = 0.0
        for i in range(self.n):
            w = float(i + 1)
            total += self.buf[(self.idx + i) % self.n] * w
            wsum += w
        return total / wsum
    def reset(self): self.__init__(self.n)


class STD(Op):
    """Rolling sample standard deviation (from buffer)."""
    def __init__(self, n: int):
        self.n = n
        self.buf = [0.0] * n
        self.idx = 0
        self.count = 0
    def warmup(self): return self.n
    def eval(self, v):
        self.buf[self.idx] = v
        self.idx = (self.idx + 1) % self.n
        if self.count < self.n: self.count += 1
        if self.count < self.n: return math.nan
        mean = sum(self.buf) / self.n
        var = sum((x - mean) ** 2 for x in self.buf) / self.n
        return math.sqrt(max(var, 0.0))
    def reset(self): self.__init__(self.n)


class VAR(Op):
    """Rolling variance (std²)."""
    def __init__(self, n: int):
        self._std = STD(n)
    def warmup(self): return self._std.warmup()
    def eval(self, v):
        s = self._std.eval(v)
        if math.isnan(s): return math.nan
        return s * s
    def reset(self): self._std.reset()


class Min(Op):
    def __init__(self, n): self.n = n; self.buf = [0.0] * n; self.idx = 0; self.count = 0
    def warmup(self): return self.n
    def eval(self, v):
        self.buf[self.idx] = v; self.idx = (self.idx + 1) % self.n
        if self.count < self.n: self.count += 1
        if self.count < self.n: return math.nan
        return min(self.buf)
    def reset(self): self.__init__(self.n)


class Max(Op):
    def __init__(self, n): self.n = n; self.buf = [0.0] * n; self.idx = 0; self.count = 0
    def warmup(self): return self.n
    def eval(self, v):
        self.buf[self.idx] = v; self.idx = (self.idx + 1) % self.n
        if self.count < self.n: self.count += 1
        if self.count < self.n: return math.nan
        return max(self.buf)
    def reset(self): self.__init__(self.n)


class Sum(Op):
    def __init__(self, n): self.n = n; self.buf = [0.0] * n; self.idx = 0; self.count = 0; self.sum = 0.0
    def warmup(self): return self.n
    def eval(self, v):
        old = self.buf[self.idx]; self.buf[self.idx] = v
        self.idx = (self.idx + 1) % self.n; self.sum += v - old
        if self.count < self.n: self.count += 1
        if self.count < self.n: return math.nan
        return self.sum
    def reset(self): self.__init__(self.n)


class Ref(Op):
    """Value from n periods ago."""
    def __init__(self, n: int): self.n = n; self.buf = [0.0] * (n + 1); self.idx = 0; self.count = 0
    def warmup(self): return self.n
    def eval(self, v):
        self.buf[self.idx] = v; self.idx = (self.idx + 1) % (self.n + 1)
        if self.count <= self.n: self.count += 1
        if self.count <= self.n: return math.nan
        return self.buf[(self.idx - self.n - 1) % (self.n + 1)]
    def reset(self): self.__init__(self.n)


class Delta(Op):
    """x - ref(x, n)."""
    def __init__(self, n: int): self._ref = Ref(n)
    def warmup(self): return self._ref.warmup()
    def eval(self, v):
        past = self._ref.eval(v)
        if math.isnan(past): return math.nan
        return v - past
    def reset(self): self._ref.reset()


class PctChange(Op):
    """x / ref(x, n) - 1."""
    def __init__(self, n: int): self._ref = Ref(n)
    def warmup(self): return self._ref.warmup()
    def eval(self, v):
        past = self._ref.eval(v)
        if math.isnan(past) or past == 0: return math.nan
        return v / past - 1
    def reset(self): self._ref.reset()


class ZScore(Op):
    """(x - sma) / std."""
    def __init__(self, n: int): self._sma = SMA(n); self._std = STD(n)
    def warmup(self): return max(self._sma.warmup(), self._std.warmup())
    def eval(self, v):
        m = self._sma.eval(v); s = self._std.eval(v)
        if math.isnan(m) or math.isnan(s) or s == 0: return math.nan
        return (v - m) / s
    def reset(self): self._sma.reset(); self._std.reset()


class Rank(Op):
    """Rolling percentile rank: count(values <= v) / n."""
    def __init__(self, n: int): self.n = n; self.buf = [0.0] * n; self.idx = 0; self.count = 0
    def warmup(self): return self.n
    def eval(self, v):
        self.buf[self.idx] = v; self.idx = (self.idx + 1) % self.n
        if self.count < self.n: self.count += 1
        if self.count < self.n: return math.nan
        le = sum(1 for x in self.buf if x <= v)
        return le / self.n
    def reset(self): self.__init__(self.n)


# ═══════════════════════════════════════════════════════════════════════
# Two-series operators
# ═══════════════════════════════════════════════════════════════════════

class Corr(DualOp):
    """Rolling Pearson correlation between two series."""
    def __init__(self, n: int):
        self.n = n
        self.x_buf = [0.0] * n
        self.y_buf = [0.0] * n
        self.idx = 0
        self.count = 0
    def warmup(self): return self.n
    def eval(self, x, y):
        self.x_buf[self.idx] = x
        self.y_buf[self.idx] = y
        self.idx = (self.idx + 1) % self.n
        if self.count < self.n: self.count += 1
        if self.count < self.n: return math.nan
        sx = sy = sxy = sx2 = sy2 = 0.0
        for i in range(self.n):
            xi = self.x_buf[i]; yi = self.y_buf[i]
            sx += xi; sy += yi; sxy += xi * yi; sx2 += xi * xi; sy2 += yi * yi
        num = self.n * sxy - sx * sy
        den = math.sqrt((self.n * sx2 - sx * sx) * (self.n * sy2 - sy * sy))
        if den == 0: return math.nan
        return num / den
    def reset(self): self.__init__(self.n)


class Cov(DualOp):
    """Rolling covariance between two series."""
    def __init__(self, n: int):
        self.n = n
        self.x_buf = [0.0] * n
        self.y_buf = [0.0] * n
        self.idx = 0
        self.count = 0
    def warmup(self): return self.n
    def eval(self, x, y):
        self.x_buf[self.idx] = x
        self.y_buf[self.idx] = y
        self.idx = (self.idx + 1) % self.n
        if self.count < self.n: self.count += 1
        if self.count < self.n: return math.nan
        mean_x = sum(self.x_buf) / self.n
        mean_y = sum(self.y_buf) / self.n
        cov = sum((self.x_buf[i] - mean_x) * (self.y_buf[i] - mean_y) for i in range(self.n))
        return cov / self.n
    def reset(self): self.__init__(self.n)


class CrossUp(DualOp):
    """Returns 1.0 when x crosses above y, 0.0 otherwise."""
    def __init__(self):
        self._init = False
        self._prev_x = 0.0
        self._prev_y = 0.0
    def warmup(self): return 1
    def eval(self, x, y):
        if not self._init:
            self._init = True; self._prev_x = x; self._prev_y = y
            return 0.0
        result = 1.0 if self._prev_x <= self._prev_y and x > y else 0.0
        self._prev_x = x; self._prev_y = y
        return result
    def reset(self): self.__init__()


class CrossDown(DualOp):
    """Returns 1.0 when x crosses below y, 0.0 otherwise."""
    def __init__(self):
        self._init = False
        self._prev_x = 0.0
        self._prev_y = 0.0
    def warmup(self): return 1
    def eval(self, x, y):
        if not self._init:
            self._init = True; self._prev_x = x; self._prev_y = y
            return 0.0
        result = 1.0 if self._prev_x >= self._prev_y and x < y else 0.0
        self._prev_x = x; self._prev_y = y
        return result
    def reset(self): self.__init__()


# ═══════════════════════════════════════════════════════════════════════
# Oscillators
# ═══════════════════════════════════════════════════════════════════════

class RSI(Op):
    def __init__(self, n): self.n = n; self.avg_gain = 0.0; self.avg_loss = 0.0; self.prev = 0.0; self.count = 0
    def warmup(self): return self.n + 1
    def eval(self, v):
        if self.count == 0: self.prev = v; self.count += 1; return math.nan
        change = v - self.prev; self.prev = v
        gain = max(change, 0.0); loss = max(-change, 0.0)
        if self.count <= self.n:
            self.avg_gain += gain; self.avg_loss += loss; self.count += 1
            if self.count <= self.n: return math.nan
            self.avg_gain /= self.n; self.avg_loss /= self.n
        else:
            self.avg_gain = (self.avg_gain * (self.n - 1) + gain) / self.n
            self.avg_loss = (self.avg_loss * (self.n - 1) + loss) / self.n; self.count += 1
        if self.avg_loss == 0: return 100.0
        return 100.0 - 100.0 / (1.0 + self.avg_gain / self.avg_loss)
    def reset(self): self.__init__(self.n)


class MACD(Op):
    def __init__(self, fast, slow): self.fast = EMA(fast); self.slow = EMA(slow)
    def warmup(self): return self.slow.warmup()
    def eval(self, v):
        f = self.fast.eval(v); s = self.slow.eval(v)
        if math.isnan(f) or math.isnan(s): return math.nan
        return f - s
    def reset(self): self.fast.reset(); self.slow.reset()


class ATR(Op):
    def __init__(self, n): self._tr = EMA(n); self._prev = 0.0; self._init = False
    def warmup(self): return self._tr.warmup() + 1
    def eval(self, v):
        if not self._init: self._init = True; self._prev = v; return math.nan
        tr = abs(v - self._prev); self._prev = v
        return self._tr.eval(tr)
    def reset(self): self._tr.reset(); self._init = False


# ═══════════════════════════════════════════════════════════════════════
# Compiler
# ═══════════════════════════════════════════════════════════════════════

def compile_expr(node: Node, fields: dict[str, int], factors: dict[str, Op] | None = None) -> Op:
    """Compile an AST node into an evaluable Op tree."""
    factors = factors or {}
    return _compile_node(node, fields, factors)


def _compile_node(node: Node, fields: dict[str, int], factors: dict[str, Op]) -> Op:
    """Compile a single AST node.  Delegates leaf / composite to helpers for low complexity."""
    if isinstance(node, (NumberLit, BoolLit, StringLit)):
        return _compile_literal(node)
    if isinstance(node, FieldRef):
        return _compile_field(node, fields)
    if isinstance(node, FactorRef):
        return _compile_factor_ref(node, factors)
    if isinstance(node, UnaryExpr):
        return _Unary(node.op, _compile_node(node.expr, fields, factors))
    if isinstance(node, BinaryExpr):
        return _compile_binary(node, fields, factors)
    if isinstance(node, TernaryExpr):
        return _compile_ternary(node, fields, factors)
    if isinstance(node, CallExpr):
        return _compile_call(node, fields, factors)
    raise TypeError(f"unknown node type {type(node)}")


def _compile_literal(node: Node) -> Op:
    if isinstance(node, NumberLit):
        return _Const(node.value)
    if isinstance(node, BoolLit):
        return _Const(1.0 if node.value else 0.0)
    raise TypeError("string literal not supported in expression context")


def _compile_field(node: FieldRef, fields: dict[str, int]) -> Op:
    if node.name not in fields:
        raise NameError(f"unknown field ${node.name}")
    return _Field()


def _compile_factor_ref(node: FactorRef, factors: dict[str, Op]) -> Op:
    if node.name not in factors:
        raise NameError(f"unknown factor {node.name!r}")
    return factors[node.name]


def _compile_binary(node: BinaryExpr, fields: dict[str, int], factors: dict[str, Op]) -> Op:
    return _Binary(
        node.op,
        _compile_node(node.left, fields, factors),
        _compile_node(node.right, fields, factors),
    )


def _compile_ternary(node: TernaryExpr, fields: dict[str, int], factors: dict[str, Op]) -> Op:
    return _Ternary(
        _compile_node(node.cond, fields, factors),
        _compile_node(node.true_expr, fields, factors),
        _compile_node(node.false_expr, fields, factors),
    )


# ── Per-operator compilers (split for complexity) ──

_WINDOW_OPS: dict[str, type] = {
    "sma": SMA, "ema": EMA, "wma": WMA, "std": STD, "var": VAR,
    "min": Min, "max": Max, "sum": Sum, "ref": Ref,
    "delta": Delta, "pct_change": PctChange, "zscore": ZScore, "rank": Rank,
    "rsi": RSI, "atr": ATR,
}
_SCALAR_OPS = frozenset({"abs", "sign", "log", "exp", "sqrt"})
_TWO_ARG_OPS = frozenset({"corr", "cov", "cross_up", "cross_down"})


def _compile_call(node: CallExpr, fields: dict[str, int], factors: dict[str, Op] | None) -> Op:
    name = node.name.lower()

    if name in _WINDOW_OPS:
        return _compile_window_op(name, node, fields, factors)

    if name == "macd":
        return _compile_macd(node, fields, factors)

    if name in _SCALAR_OPS:
        return _compile_scalar(name, node, fields, factors)

    if name == "pow":
        return _compile_pow(node, fields, factors)

    if name in ("bb_upper", "bb_lower"):
        return _compile_bb(name, node, fields, factors)

    if name == "if_":
        return _compile_if(node, fields, factors)

    if name in _TWO_ARG_OPS:
        return _compile_two_arg(name, node, fields, factors)

    raise NameError(f"unknown function {name!r}")


# Operators that implicitly use $close when called with 1 arg (just the period)
_IMPLICIT_CLOSE_OPS = frozenset({"rsi", "atr"})


def _compile_window_op(name: str, node: CallExpr, fields, factors) -> Op:
    if name in _IMPLICIT_CLOSE_OPS and len(node.args) == 1:
        # rsi(14) or atr(14) — implicit $close field, period = args[0]
        inner = _Field()
        n = int(node.args[0].value) if isinstance(node.args[0], NumberLit) else 14
    else:
        inner, n = _compile_window_args(node, fields, factors)
    return _Wrap(inner, _WINDOW_OPS[name](n))


def _compile_macd(node: CallExpr, fields, factors) -> Op:
    inner, _ = _compile_window_args(node, fields, factors)
    fast = _arg_int(node.args, 0, 12)
    slow = _arg_int(node.args, 1, 26)
    return _Wrap(inner, MACD(fast, slow))


def _compile_scalar(name: str, node: CallExpr, fields, factors) -> Op:
    inner = compile_expr(node.args[0], fields, factors)
    return _Scalar(name, inner)


def _compile_pow(node: CallExpr, fields, factors) -> Op:
    inner = compile_expr(node.args[0], fields, factors)
    exp = _arg_float(node.args, 0, 2.0)
    return _Pow(inner, exp)


def _compile_bb(name: str, node: CallExpr, fields, factors) -> Op:
    inner = compile_expr(node.args[0], fields, factors)
    n = _arg_int(node.args, 0, 20)
    k = _arg_float(node.args, 1, 2.0)
    return _Wrap(inner, _BB(n, k, upper=(name == "bb_upper")))


def _compile_if(node: CallExpr, fields, factors) -> Op:
    if len(node.args) != 3:
        raise TypeError("if_ requires 3 arguments: condition, true_expr, false_expr")
    return _Ternary(
        compile_expr(node.args[0], fields, factors),
        compile_expr(node.args[1], fields, factors),
        compile_expr(node.args[2], fields, factors),
    )


def _compile_two_arg(name: str, node: CallExpr, fields, factors) -> Op:
    if len(node.args) < 2:
        raise TypeError(f"{name} requires at least 2 arguments")
    inner_x = compile_expr(node.args[0], fields, factors)
    inner_y = compile_expr(node.args[1], fields, factors)
    if name in ("corr", "cov"):
        n = _arg_int(node.args, 1, 14)
        cls = Corr if name == "corr" else Cov
        return _Dual(inner_x, inner_y, cls(n))
    cls = CrossUp if name == "cross_up" else CrossDown
    return _Dual(inner_x, inner_y, cls())


def _compile_window_args(node: CallExpr, fields, factors) -> tuple[Op, int]:
    """Extract (inner_op, window_n) for single-series window functions."""
    inner = compile_expr(node.args[0], fields, factors)
    n = _arg_int(node.args, 0, 14)
    return inner, n


# ═══════════════════════════════════════════════════════════════════════
# Wrapper operators
# ═══════════════════════════════════════════════════════════════════════

class _Const(Op):
    def __init__(self, v): self.v = float(v)
    def eval(self, _): return self.v


class _Field(Op):
    def eval(self, v): return v


class _Unary(Op):
    def __init__(self, op, inner): self.op = op; self.inner = inner
    def eval(self, v):
        x = self.inner.eval(v)
        if self.op == "-": return -x
        if self.op == "!": return 1.0 if x == 0 else 0.0
        return math.nan
    def reset(self): self.inner.reset()


class _Binary(Op):
    def __init__(self, op, left, right): self.op = op; self.left = left; self.right = right
    def eval(self, v):
        l = self.left.eval(v); r = self.right.eval(v)
        return _BINARY_DISPATCH(self.op, l, r)
    def reset(self): self.left.reset(); self.right.reset()


def _BINARY_DISPATCH(op: str, l: float, r: float) -> float:
    """Dispatch binary operator (lazy lambdas — avoids eager div/mod by zero)."""
    return _BIN_OPS.get(op, lambda a, b: math.nan)(l, r)


_BIN_OPS: dict[str, object] = {
    "+": lambda a, b: a + b,
    "-": lambda a, b: a - b,
    "*": lambda a, b: a * b,
    "/": lambda a, b: math.nan if b == 0 else a / b,
    "%": lambda a, b: math.fmod(a, b) if b != 0 else math.nan,
    "==": lambda a, b: 1.0 if a == b else 0.0,
    "!=": lambda a, b: 1.0 if a != b else 0.0,
    "<": lambda a, b: 1.0 if a < b else 0.0,
    "<=": lambda a, b: 1.0 if a <= b else 0.0,
    ">": lambda a, b: 1.0 if a > b else 0.0,
    ">=": lambda a, b: 1.0 if a >= b else 0.0,
    "&&": lambda a, b: 1.0 if a != 0 and b != 0 else 0.0,
    "||": lambda a, b: 1.0 if a != 0 or b != 0 else 0.0,
}


class _Ternary(Op):
    def __init__(self, cond, t, f): self.cond = cond; self.t = t; self.f = f
    def eval(self, v): return self.t.eval(v) if self.cond.eval(v) != 0 else self.f.eval(v)
    def reset(self): self.cond.reset(); self.t.reset(); self.f.reset()


class _Wrap(Op):
    def __init__(self, inner, outer): self.inner = inner; self.outer = outer
    def eval(self, v):
        x = self.inner.eval(v)
        if math.isnan(x): return math.nan
        return self.outer.eval(x)
    def reset(self): self.inner.reset(); self.outer.reset()


class _Scalar(Op):
    def __init__(self, name, inner): self.name = name; self.inner = inner
    def eval(self, v):
        x = self.inner.eval(v)
        fns = {"abs": abs, "sign": lambda a: 1.0 if a > 0 else (-1.0 if a < 0 else 0.0),
               "log": lambda a: math.nan if a <= 0 else math.log(a), "exp": math.exp,
               "sqrt": lambda a: math.nan if a < 0 else math.sqrt(a)}
        return fns[self.name](x)
    def reset(self): self.inner.reset()


class _Pow(Op):
    def __init__(self, inner, exp): self.inner = inner; self.exp = exp
    def eval(self, v):
        x = self.inner.eval(v)
        if math.isnan(x): return math.nan
        try: return math.pow(x, self.exp)
        except (ValueError, OverflowError): return math.nan
    def reset(self): self.inner.reset()


class _BB(Op):
    def __init__(self, n, k, upper):
        self._sma = SMA(n); self._std = STD(n); self.k = k; self._upper = upper
    def warmup(self): return self._sma.warmup()
    def eval(self, v):
        m = self._sma.eval(v); s = self._std.eval(v)
        if math.isnan(m) or math.isnan(s): return math.nan
        return m + self.k * s if self._upper else m - self.k * s
    def reset(self): self._sma.reset(); self._std.reset()


class _Dual(Op):
    """Adapter that evaluates two inner Op trees and feeds them to a DualOp."""
    def __init__(self, inner_x: Op, inner_y: Op, dual: DualOp):
        self.left = inner_x; self.right = inner_y; self.dual = dual
    def eval(self, v):
        x = self.left.eval(v); y = self.right.eval(v)
        if math.isnan(x) or math.isnan(y): return math.nan
        return self.dual.eval(x, y)
    def warmup(self):
        return max(self.left.warmup() if hasattr(self.left, 'warmup') else 0,
                   self.right.warmup() if hasattr(self.right, 'warmup') else 0,
                   self.dual.warmup())
    def reset(self): self.left.reset(); self.right.reset(); self.dual.reset()


# ═══════════════════════════════════════════════════════════════════════
# Helpers
# ═══════════════════════════════════════════════════════════════════════

def _arg_int(args: list[Node], idx: int, default: int) -> int:
    if idx + 1 >= len(args): return default
    v = args[idx + 1]
    if isinstance(v, NumberLit): return int(v.value)
    return default


def _arg_float(args: list[Node], idx: int, default: float) -> float:
    if idx + 1 >= len(args): return default
    v = args[idx + 1]
    if isinstance(v, NumberLit): return v.value
    return default
