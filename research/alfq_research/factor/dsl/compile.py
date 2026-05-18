"""ALFQ factor DSL — streaming compiler and operators.

Maps DSL AST → evaluable operator tree that consumes bar-by-bar float values.
All 22 operators from docs/09 with identical semantics to the Go engine.
"""

import math
from abc import ABC, abstractmethod
from .ast_ import *


# ── Operator interface ──

class Op(ABC):
    @abstractmethod
    def eval(self, v: float) -> float: ...
    def reset(self): pass
    def warmup(self) -> int: return 0


# ── Window / moving average operators ──

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

class STD(Op):
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
        return math.sqrt(var) if var >= 0 else 0.0
    def reset(self): self.__init__(self.n)

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
    def __init__(self, n: int): self.n = n; self.buf = [0.0] * (n + 1); self.idx = 0; self.count = 0
    def warmup(self): return self.n
    def eval(self, v):
        self.buf[self.idx] = v; self.idx = (self.idx + 1) % (self.n + 1)
        if self.count <= self.n: self.count += 1
        if self.count <= self.n: return math.nan
        return self.buf[(self.idx - self.n - 1) % (self.n + 1)]
    def reset(self): self.__init__(self.n)

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
    def __init__(self, n): self.tr = EMA(n); self.prev = 0.0; self.init = False
    def warmup(self): return self.tr.warmup() + 1
    def eval(self, v):
        if not self.init: self.init = True; self.prev = v; return math.nan
        tr = abs(v - self.prev); self.prev = v
        return self.tr.eval(tr)
    def reset(self): self.tr.reset(); self.init = False


# ── Compiler ──

def compile_expr(node: Node, fields: dict[str, int], factors: dict[str, Op] | None = None) -> Op:
    """Compile an AST node into an evaluable Op tree."""
    factors = factors or {}

    if isinstance(node, NumberLit):
        return _Const(node.value)
    if isinstance(node, FieldRef):
        if node.name not in fields:
            raise NameError(f"unknown field ${node.name}")
        return _Field()
    if isinstance(node, FactorRef):
        if node.name not in factors:
            raise NameError(f"unknown factor {node.name!r}")
        return factors[node.name]
    if isinstance(node, UnaryExpr):
        inner = compile_expr(node.expr, fields, factors)
        return _Unary(node.op, inner)
    if isinstance(node, BinaryExpr):
        return _Binary(node.op, compile_expr(node.left, fields, factors), compile_expr(node.right, fields, factors))
    if isinstance(node, TernaryExpr):
        return _Ternary(compile_expr(node.cond, fields, factors), compile_expr(node.true_expr, fields, factors), compile_expr(node.false_expr, fields, factors))
    if isinstance(node, CallExpr):
        return _compile_call(node, fields, factors)
    raise TypeError(f"unknown node type {type(node)}")

def _compile_call(node: CallExpr, fields, factors) -> Op:
    name = node.name.lower()
    ops_map = {
        "sma": SMA, "ema": EMA, "std": STD, "min": Min, "max": Max, "sum": Sum,
        "ref": Ref, "rsi": RSI, "atr": ATR,
    }
    if name in ops_map:
        inner = compile_expr(node.args[0], fields, factors)
        n = int(node.args[1].value) if len(node.args) > 1 and isinstance(node.args[1], NumberLit) else 14
        return _Wrap(inner, ops_map[name](n))
    if name == "macd":
        inner = compile_expr(node.args[0], fields, factors)
        fast = int(node.args[1].value) if len(node.args) > 1 and isinstance(node.args[1], NumberLit) else 12
        slow = int(node.args[2].value) if len(node.args) > 2 and isinstance(node.args[2], NumberLit) else 26
        return _Wrap(inner, MACD(fast, slow))
    if name in ("abs", "sign", "log", "exp", "sqrt"):
        inner = compile_expr(node.args[0], fields, factors)
        return _Scalar(name, inner)
    if name in ("bb_upper", "bb_lower"):
        inner = compile_expr(node.args[0], fields, factors)
        n = int(node.args[1].value) if len(node.args) > 1 and isinstance(node.args[1], NumberLit) else 20
        k = float(node.args[2].value) if len(node.args) > 2 and isinstance(node.args[2], NumberLit) else 2.0
        outer = _BB(n, k, upper=(name == "bb_upper"))
        return _Wrap(inner, outer)
    raise NameError(f"unknown function {name!r}")


# ── Wrapper operators ──

class _Const(Op):
    def __init__(self, v): self.v = v
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
        ops = {"+": l + r, "-": l - r, "*": l * r, "/": (math.nan if r == 0 else l / r),
               "%": math.fmod(l, r), "==": 1.0 if l == r else 0.0, "!=": 1.0 if l != r else 0.0,
               "<": 1.0 if l < r else 0.0, "<=": 1.0 if l <= r else 0.0,
               ">": 1.0 if l > r else 0.0, ">=": 1.0 if l >= r else 0.0,
               "&&": 1.0 if l != 0 and r != 0 else 0.0, "||": 1.0 if l != 0 or r != 0 else 0.0}
        return ops.get(self.op, math.nan)
    def reset(self): self.left.reset(); self.right.reset()

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

class _BB(Op):
    def __init__(self, n, k, upper):
        self.sma = SMA(n); self.std = STD(n); self.k = k; self.upper = upper
    def warmup(self): return self.sma.warmup()
    def eval(self, v):
        m = self.sma.eval(v); s = self.std.eval(v)
        if math.isnan(m) or math.isnan(s): return math.nan
        return m + self.k * s if self.upper else m - self.k * s
    def reset(self): self.sma.reset(); self.std.reset()
