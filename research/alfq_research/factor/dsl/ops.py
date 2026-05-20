"""ALFQ factor DSL — operator implementations (streaming, bar-by-bar).

Every operator matches the Go engine semantics from backend/go/internal/factor/dsl/.
"""

from __future__ import annotations

import math
from abc import ABC, abstractmethod


# ═══════════════════════════════════════════════════════════════════════
# Operator interfaces
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
# Moving average operators
# ═══════════════════════════════════════════════════════════════════════

class SMA(Op):
    def __init__(self, n: int):
        self.n = n; self.buf = [0.0] * n; self.idx = 0; self.sum = 0.0; self.count = 0
    def warmup(self): return self.n
    def eval(self, v):
        old = self.buf[self.idx]; self.buf[self.idx] = v
        self.idx = (self.idx + 1) % self.n; self.sum += v - old
        if self.count < self.n: self.count += 1
        if self.count < self.n: return math.nan
        return self.sum / self.n
    def reset(self): self.__init__(self.n)


class EMA(Op):
    def __init__(self, n: int):
        self.n = n; self.alpha = 2.0 / (n + 1); self.value = 0.0; self.count = 0
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
        self.n = n; self.buf = [0.0] * n; self.idx = 0; self.count = 0
    def warmup(self): return self.n
    def eval(self, v):
        self.buf[self.idx] = v; self.idx = (self.idx + 1) % self.n
        if self.count < self.n: self.count += 1
        if self.count < self.n: return math.nan
        total, wsum = 0.0, 0.0
        for i in range(self.n):
            w = float(i + 1)
            total += self.buf[(self.idx + i) % self.n] * w
            wsum += w
        return total / wsum
    def reset(self): self.__init__(self.n)


# ═══════════════════════════════════════════════════════════════════════
# Statistics operators
# ═══════════════════════════════════════════════════════════════════════

class STD(Op):
    """Rolling sample standard deviation."""
    def __init__(self, n: int):
        self.n = n; self.buf = [0.0] * n; self.idx = 0; self.count = 0
    def warmup(self): return self.n
    def eval(self, v):
        self.buf[self.idx] = v; self.idx = (self.idx + 1) % self.n
        if self.count < self.n: self.count += 1
        if self.count < self.n: return math.nan
        mean = sum(self.buf) / self.n
        var = sum((x - mean) ** 2 for x in self.buf) / self.n
        return math.sqrt(max(var, 0.0))
    def reset(self): self.__init__(self.n)


class VAR(Op):
    """Rolling variance (std²)."""
    def __init__(self, n: int): self._std = STD(n)
    def warmup(self): return self._std.warmup()
    def eval(self, v):
        s = self._std.eval(v)
        return math.nan if math.isnan(s) else s * s
    def reset(self): self._std.reset()


class Min(Op):
    def __init__(self, n): self.n = n; self.buf = [0.0] * n; self.idx = 0; self.count = 0
    def warmup(self): return self.n
    def eval(self, v):
        self.buf[self.idx] = v; self.idx = (self.idx + 1) % self.n
        if self.count < self.n: self.count += 1
        return math.nan if self.count < self.n else min(self.buf)
    def reset(self): self.__init__(self.n)


class Max(Op):
    def __init__(self, n): self.n = n; self.buf = [0.0] * n; self.idx = 0; self.count = 0
    def warmup(self): return self.n
    def eval(self, v):
        self.buf[self.idx] = v; self.idx = (self.idx + 1) % self.n
        if self.count < self.n: self.count += 1
        return math.nan if self.count < self.n else max(self.buf)
    def reset(self): self.__init__(self.n)


class Sum(Op):
    def __init__(self, n): self.n = n; self.buf = [0.0] * n; self.idx = 0; self.count = 0; self.sum = 0.0
    def warmup(self): return self.n
    def eval(self, v):
        old = self.buf[self.idx]; self.buf[self.idx] = v
        self.idx = (self.idx + 1) % self.n; self.sum += v - old
        if self.count < self.n: self.count += 1
        return math.nan if self.count < self.n else self.sum
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
        return math.nan if math.isnan(past) else v - past
    def reset(self): self._ref.reset()


class PctChange(Op):
    """x / ref(x, n) - 1."""
    def __init__(self, n: int): self._ref = Ref(n)
    def warmup(self): return self._ref.warmup()
    def eval(self, v):
        past = self._ref.eval(v)
        return math.nan if math.isnan(past) or past == 0 else v / past - 1
    def reset(self): self._ref.reset()


class ZScore(Op):
    """(x - sma) / std."""
    def __init__(self, n: int): self._sma = SMA(n); self._std = STD(n)
    def warmup(self): return max(self._sma.warmup(), self._std.warmup())
    def eval(self, v):
        m = self._sma.eval(v); s = self._std.eval(v)
        return math.nan if math.isnan(m) or math.isnan(s) or s == 0 else (v - m) / s
    def reset(self): self._sma.reset(); self._std.reset()


class Rank(Op):
    """Rolling percentile rank: count(values <= v) / n."""
    def __init__(self, n: int): self.n = n; self.buf = [0.0] * n; self.idx = 0; self.count = 0
    def warmup(self): return self.n
    def eval(self, v):
        self.buf[self.idx] = v; self.idx = (self.idx + 1) % self.n
        if self.count < self.n: self.count += 1
        if self.count < self.n: return math.nan
        return sum(1 for x in self.buf if x <= v) / self.n
    def reset(self): self.__init__(self.n)


# ═══════════════════════════════════════════════════════════════════════
# Correlation / covariance (two-series)
# ═══════════════════════════════════════════════════════════════════════

class Corr(DualOp):
    """Rolling Pearson correlation between two series."""
    def __init__(self, n: int):
        self.n = n; self.x_buf = [0.] * n; self.y_buf = [0.] * n; self.idx = 0; self.count = 0
    def warmup(self): return self.n
    def eval(self, x, y):
        self.x_buf[self.idx] = x; self.y_buf[self.idx] = y
        self.idx = (self.idx + 1) % self.n
        if self.count < self.n: self.count += 1
        if self.count < self.n: return math.nan
        sx = sy = sxy = sx2 = sy2 = 0.0
        for i in range(self.n):
            xi, yi = self.x_buf[i], self.y_buf[i]
            sx += xi; sy += yi; sxy += xi * yi; sx2 += xi * xi; sy2 += yi * yi
        num = self.n * sxy - sx * sy
        den = math.sqrt((self.n * sx2 - sx * sx) * (self.n * sy2 - sy * sy))
        return math.nan if den == 0 else num / den
    def reset(self): self.__init__(self.n)


class Cov(DualOp):
    """Rolling covariance between two series."""
    def __init__(self, n: int):
        self.n = n; self.x_buf = [0.] * n; self.y_buf = [0.] * n; self.idx = 0; self.count = 0
    def warmup(self): return self.n
    def eval(self, x, y):
        self.x_buf[self.idx] = x; self.y_buf[self.idx] = y
        self.idx = (self.idx + 1) % self.n
        if self.count < self.n: self.count += 1
        if self.count < self.n: return math.nan
        mx = sum(self.x_buf) / self.n; my = sum(self.y_buf) / self.n
        return sum((self.x_buf[i] - mx) * (self.y_buf[i] - my) for i in range(self.n)) / self.n
    def reset(self): self.__init__(self.n)


class CrossUp(DualOp):
    """Returns 1.0 when x crosses above y, 0.0 otherwise."""
    def __init__(self):
        self._init = False; self._px = 0.0; self._py = 0.0
    def warmup(self): return 1
    def eval(self, x, y):
        if not self._init:
            self._init = True; self._px = x; self._py = y; return 0.0
        result = 1.0 if self._px <= self._py and x > y else 0.0
        self._px = x; self._py = y; return result
    def reset(self): self.__init__()


class CrossDown(DualOp):
    """Returns 1.0 when x crosses below y, 0.0 otherwise."""
    def __init__(self):
        self._init = False; self._px = 0.0; self._py = 0.0
    def warmup(self): return 1
    def eval(self, x, y):
        if not self._init:
            self._init = True; self._px = x; self._py = y; return 0.0
        result = 1.0 if self._px >= self._py and x < y else 0.0
        self._px = x; self._py = y; return result
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
        return math.nan if math.isnan(f) or math.isnan(s) else f - s
    def reset(self): self.fast.reset(); self.slow.reset()


class ATR(Op):
    def __init__(self, n): self._tr = EMA(n); self._prev = 0.0; self._init = False
    def warmup(self): return self._tr.warmup() + 1
    def eval(self, v):
        if not self._init: self._init = True; self._prev = v; return math.nan
        tr = abs(v - self._prev); self._prev = v
        return self._tr.eval(tr)
    def reset(self): self._tr.reset(); self._init = False
