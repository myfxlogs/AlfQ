"""Parity test: Python streaming DSL vs expected values.

Verifies ALL 22+ DSL operators produce results consistent with the Go engine.
Uses golden data generated from known expression + bar sequence pairs.

Target: ≥ 5000 assertions across all operators.
"""

from __future__ import annotations

import math
import random

import pytest

from alfq_research.factor.dsl.parser import parse
from alfq_research.factor.dsl.compile import compile_expr


# ═══════════════════════════════════════════════════════════════════
# Test infrastructure
# ═══════════════════════════════════════════════════════════════════

FIELDS = {"close": 0, "open": 1, "high": 2, "low": 3, "volume": 4}


def eval_expr(expr: str, bars: list[float], *, fields: dict[str, int] | None = None) -> list[float]:
    node = parse(expr)
    op = compile_expr(node, fields or FIELDS)
    return [op.eval(v) for v in bars]


def assert_close(actual: list[float], expected: list[float], *, tol: float = 1e-9):
    """Assert two float lists match element-wise, handling NaN."""
    assert len(actual) == len(expected), f"length mismatch: {len(actual)} vs {len(expected)}"
    for i, (a, e) in enumerate(zip(actual, expected)):
        if math.isnan(e):
            assert math.isnan(a), f"bar {i}: expected NaN, got {a}"
        else:
            assert abs(a - e) < tol, f"bar {i}: expected {e}, got {a}"


# ═══════════════════════════════════════════════════════════════════
# Bar generators
# ═══════════════════════════════════════════════════════════════════

def lin_bars(n: int, start: float = 100.0, step: float = 1.0) -> list[float]:
    return [start + i * step for i in range(n)]


def const_bars(n: int, val: float = 100.0) -> list[float]:
    return [val] * n


def sine_bars(n: int, amp: float = 10.0, base: float = 100.0) -> list[float]:
    return [base + amp * math.sin(2 * math.pi * i / n) for i in range(n)]


def rand_bars(n: int, seed: int = 42, low: float = 90.0, high: float = 110.0) -> list[float]:
    rng = random.Random(seed)
    return [low + rng.random() * (high - low) for _ in range(n)]


# ═══════════════════════════════════════════════════════════════════
# Golden value generators
# ═══════════════════════════════════════════════════════════════════

def golden_sma(bars: list[float], n: int) -> list[float]:
    result, buf = [], []
    for v in bars:
        buf.append(v)
        if len(buf) > n: buf.pop(0)
        result.append(math.nan if len(buf) < n else sum(buf) / n)
    return result


def golden_ema(bars: list[float], n: int) -> list[float]:
    result, alpha, ema = [], 2.0 / (n + 1), 0.0
    for i, v in enumerate(bars):
        ema = v if i == 0 else alpha * v + (1 - alpha) * ema
        result.append(math.nan if i < n - 1 else ema)
    return result


def golden_wma(bars: list[float], n: int) -> list[float]:
    result, buf = [], []
    for v in bars:
        buf.append(v)
        if len(buf) > n: buf.pop(0)
        if len(buf) < n: result.append(math.nan)
        else:
            wsum = sum(b * (i + 1) for i, b in enumerate(buf))
            result.append(wsum / (n * (n + 1) // 2))
    return result


def golden_std(bars: list[float], n: int) -> list[float]:
    result, buf = [], []
    for v in bars:
        buf.append(v)
        if len(buf) > n: buf.pop(0)
        if len(buf) < n: result.append(math.nan)
        else:
            mean = sum(buf) / n
            var = sum((x - mean) ** 2 for x in buf) / n
            result.append(math.sqrt(max(var, 0.0)))
    return result


def golden_var(bars: list[float], n: int) -> list[float]:
    s = golden_std(bars, n)
    return [math.nan if math.isnan(x) else x * x for x in s]


def golden_min(bars: list[float], n: int) -> list[float]:
    result, buf = [], []
    for v in bars:
        buf.append(v)
        if len(buf) > n: buf.pop(0)
        result.append(math.nan if len(buf) < n else min(buf))
    return result


def golden_max(bars: list[float], n: int) -> list[float]:
    result, buf = [], []
    for v in bars:
        buf.append(v)
        if len(buf) > n: buf.pop(0)
        result.append(math.nan if len(buf) < n else max(buf))
    return result


def golden_sum(bars: list[float], n: int) -> list[float]:
    result, buf = [], []
    for v in bars:
        buf.append(v)
        if len(buf) > n: buf.pop(0)
        result.append(math.nan if len(buf) < n else sum(buf))
    return result


def golden_ref(bars: list[float], n: int) -> list[float]:
    result, buf = [], []
    for v in bars:
        buf.append(v)
        result.append(math.nan if len(buf) <= n else buf[-(n + 1)])
    return result


def golden_delta(bars: list[float], n: int) -> list[float]:
    refs = golden_ref(bars, n)
    return [math.nan if math.isnan(r) else bars[i] - r for i, r in enumerate(refs)]


def golden_pct_change(bars: list[float], n: int) -> list[float]:
    refs = golden_ref(bars, n)
    return [math.nan if math.isnan(r) or r == 0 else bars[i] / r - 1 for i, r in enumerate(refs)]


def golden_zscore(bars: list[float], n: int) -> list[float]:
    smas = golden_sma(bars, n)
    stds = golden_std(bars, n)
    return [
        math.nan if math.isnan(s) or math.isnan(d) or d == 0 else (bars[i] - s) / d
        for i, (s, d) in enumerate(zip(smas, stds))
    ]


def golden_rank(bars: list[float], n: int) -> list[float]:
    result, buf = [], []
    for v in bars:
        buf.append(v)
        if len(buf) > n: buf.pop(0)
        if len(buf) < n: result.append(math.nan)
        else: result.append(sum(1 for x in buf if x <= v) / n)
    return result


# ═══════════════════════════════════════════════════════════════════
# Parameterized test generation
# ═══════════════════════════════════════════════════════════════════

WINDOW_SIZES = [3, 5, 10, 20]
BAR_GENERATORS = {
    "lin_30": lin_bars(30),
    "const_30": const_bars(30),
    "sine_30": sine_bars(30),
    "rand_30": rand_bars(30),
    "lin_50": lin_bars(50),
}


def _window_param_matrix():
    return [
        ("sma($close, {n})", golden_sma),
        ("ema($close, {n})", golden_ema),
        ("wma($close, {n})", golden_wma),
        ("std($close, {n})", golden_std),
        ("var($close, {n})", golden_var),
        ("min($close, {n})", golden_min),
        ("max($close, {n})", golden_max),
        ("sum($close, {n})", golden_sum),
        ("ref($close, {n})", golden_ref),
        ("delta($close, {n})", golden_delta),
        ("pct_change($close, {n})", golden_pct_change),
        ("zscore($close, {n})", golden_zscore),
        ("rank($close, {n})", golden_rank),
    ]


# ═══════════════════════════════════════════════════════════════════
# Test classes
# ═══════════════════════════════════════════════════════════════════

class TestWindowOps:
    """Systematic coverage of all single-series window operators.

    Matrix: 13 operators × 5 bar sequences × 4 window sizes = 260 tests.
    Each test validates ~len(bars) bars (~20-48 per test depending on sequence).
    Total assertions ≈ 260 × 25 = ~6500.
    """

    @pytest.mark.parametrize("expr_tpl,golden_fn", _window_param_matrix())
    @pytest.mark.parametrize("bar_name,bars", list(BAR_GENERATORS.items()))
    @pytest.mark.parametrize("n", WINDOW_SIZES)
    def test_window_op(self, expr_tpl, golden_fn, bar_name, bars, n):
        expr = expr_tpl.format(n=n)
        result = eval_expr(expr, bars)
        expected = golden_fn(bars, n)
        assert_close(result, expected)


class TestOscillators:
    """RSI, MACD, ATR."""

    RSI_SIZES = [5, 7, 14, 21]
    OSC_BARS = {
        "lin_40": lin_bars(40),
        "sine_40": sine_bars(40),
        "rand_40": rand_bars(40),
    }

    @staticmethod
    def golden_rsi(bars: list[float], n: int) -> list[float]:
        result, prev, avg_gain, avg_loss = [], 0.0, 0.0, 0.0
        for i, v in enumerate(bars):
            if i == 0:
                prev = v; result.append(math.nan); continue
            change = v - prev; prev = v
            gain = max(change, 0); loss = max(-change, 0)
            if i <= n:
                avg_gain += gain; avg_loss += loss
                if i < n: result.append(math.nan); continue
                avg_gain /= n; avg_loss /= n
            else:
                avg_gain = (avg_gain * (n - 1) + gain) / n
                avg_loss = (avg_loss * (n - 1) + loss) / n
            result.append(100.0 if avg_loss == 0 else 100.0 - 100.0 / (1.0 + avg_gain / avg_loss))
        return result

    @pytest.mark.parametrize("bar_name,bars", list(OSC_BARS.items()))
    @pytest.mark.parametrize("n", RSI_SIZES)
    def test_rsi(self, bar_name, bars, n):
        result = eval_expr(f"rsi({n})", bars)
        expected = self.golden_rsi(bars, n)
        assert_close(result, expected)

    @pytest.mark.parametrize("bar_name,bars", list(OSC_BARS.items()))
    def test_macd(self, bar_name, bars):
        result = eval_expr("macd($close, 12, 26)", bars)
        fast = golden_ema(bars, 12)
        slow = golden_ema(bars, 26)
        expected = [
            math.nan if math.isnan(f) or math.isnan(s) else f - s
            for f, s in zip(fast, slow)
        ]
        assert_close(result, expected)

    @pytest.mark.parametrize("bar_name,bars", list(OSC_BARS.items()))
    def test_atr(self, bar_name, bars):
        """ATR(14) on close — uses abs(close[i] - close[i-1]) as TR, then EMA."""
        result = eval_expr("atr(14)", bars)
        # Manual ATR: TR[i] = abs(close[i] - close[i-1]), then EMA(14) on TR.
        tr_bars = [math.nan] + [abs(bars[i] - bars[i - 1]) for i in range(1, len(bars))]
        # Skip the first NaN TR bar, apply EMA with 14 on TRs, then prepend NaN for warmup
        ema_on_tr = golden_ema(tr_bars[1:], 14)
        # ATR warmup = 15: bar 0 = NaN (no prev), bars 1..14 = EMA warmup NaN, bar 15 = first value
        expected = [math.nan] + ema_on_tr
        assert_close(result, expected)


class TestScalarOps:
    """abs, sign, log, exp, sqrt, pow."""

    def test_abs(self):
        assert_close(eval_expr("abs($close)", [-5., -1., 0., 3., 10.]), [5., 1., 0., 3., 10.])

    def test_sign(self):
        assert_close(eval_expr("sign($close)", [-5., -1., 0., 3., 10.]), [-1., -1., 0., 1., 1.])

    def test_log(self):
        bars = [0.01, 0.5, 1.0, math.e, 100., -1.]
        expected = [math.log(0.01), math.log(0.5), 0., 1., math.log(100.), math.nan]
        assert_close(eval_expr("log($close)", bars), expected)

    def test_exp(self):
        bars = [-1., 0., 1., 2.]
        assert_close(eval_expr("exp($close)", bars), [math.exp(x) for x in bars])

    def test_sqrt(self):
        assert_close(eval_expr("sqrt($close)", [0., 1., 4., 9., -1.]), [0., 1., 2., 3., math.nan])

    def test_pow_int(self):
        bars = [1., 2., 3., 4., 0., -2.]
        for exp in [1., 2., 3.]:
            expected = [math.pow(v, exp) if v >= 0 or exp == int(exp) else math.nan for v in bars]
            assert_close(eval_expr(f"pow($close, {exp})", bars), expected)

    def test_pow_frac(self):
        """Fractional pow: sqrt via pow(x, 0.5). Negative x → NaN."""
        bars = [1., 2., 4., 0., -4.]
        expected = [1., math.sqrt(2), 2., 0., math.nan]
        assert_close(eval_expr("pow($close, 0.5)", bars), expected)


class TestBollinger:
    """BB upper and lower bands."""

    @pytest.mark.parametrize("bar_name,bars", list(BAR_GENERATORS.items()))
    def test_bb_upper(self, bar_name, bars):
        result = eval_expr("bb_upper($close, 20, 2)", bars)
        smas, stds = golden_sma(bars, 20), golden_std(bars, 20)
        expected = [math.nan if math.isnan(m) or math.isnan(s) else m + 2 * s for m, s in zip(smas, stds)]
        assert_close(result, expected)

    @pytest.mark.parametrize("bar_name,bars", list(BAR_GENERATORS.items()))
    def test_bb_lower(self, bar_name, bars):
        result = eval_expr("bb_lower($close, 20, 2)", bars)
        smas, stds = golden_sma(bars, 20), golden_std(bars, 20)
        expected = [math.nan if math.isnan(m) or math.isnan(s) else m - 2 * s for m, s in zip(smas, stds)]
        assert_close(result, expected)


class TestTwoSeriesOps:
    """corr, cov, cross_up, cross_down.

    Note: The streaming DSL eval passes a single float value v per bar to
    op.eval(v).  For dual-field expressions like corr($close, $volume, n),
    both inner _Field operators receive the same v.  This means corr/cov of
    two identical series produce either 1.0 (non-constant) or NaN (constant,
    denominator zero).  cross_up/cross_down require two genuinely different
    streams to be meaningful.
    """

    def test_corr_identical(self):
        """corr($close, $close, 10) on linear series → 1.0 after warmup."""
        bars = lin_bars(30)
        result = eval_expr("corr($close, $close, 10)", bars)
        for i, r in enumerate(result):
            if i < 9:
                assert math.isnan(r), f"bar {i}: expected NaN during warmup"
            else:
                assert abs(r - 1.0) < 1e-9, f"bar {i}: expected 1.0, got {r}"

    def test_cov_identical(self):
        """cov($close, $close, 10) on linear series → variance after warmup."""
        bars = lin_bars(30)
        result = eval_expr("cov($close, $close, 10)", bars)
        var_vals = golden_var(bars, 10)
        assert_close(result, var_vals)

    def test_corr_constant(self):
        """corr on constant series → NaN (denominator zero)."""
        bars = const_bars(20, 5.0)
        result = eval_expr("corr($close, $close, 5)", bars)
        for i, r in enumerate(result):
            if i < 4: assert math.isnan(r)
            else: assert math.isnan(r)  # constant → denom 0 → NaN

    def test_cross_up_never(self):
        """cross_up(x, x) never crosses (same series)."""
        bars = lin_bars(10)
        result = eval_expr("cross_up($close, $close)", bars)
        for r in result:
            assert r == 0.0

    def test_cross_down_never(self):
        """cross_down(x, x) never crosses (same series)."""
        bars = lin_bars(10)
        result = eval_expr("cross_down($close, $close)", bars)
        for r in result:
            assert r == 0.0


class TestBinaryOps:
    """All 12 binary operators."""

    def test_add(self):
        assert_close(eval_expr("$close + 10", lin_bars(5)), [110., 111., 112., 113., 114.])

    def test_sub(self):
        assert_close(eval_expr("$close - 5", lin_bars(5)), [95., 96., 97., 98., 99.])

    def test_mul(self):
        assert_close(eval_expr("$close * 2", lin_bars(4)), [200., 202., 204., 206.])

    def test_div(self):
        assert_close(eval_expr("$close / 2", [2., 4., 6.]), [1., 2., 3.])

    def test_div_by_zero(self):
        assert math.isnan(eval_expr("$close / 0", [5.])[0])

    def test_mod(self):
        result = eval_expr("$close % 3", [1., 2., 3., 4., 5.])
        assert_close(result, [1., 2., 0., 1., 2.])

    def test_mod_zero(self):
        assert math.isnan(eval_expr("$close % 0", [5.])[0])

    def test_eq(self):
        assert_close(eval_expr("$close == 5", [3., 5., 5., 7.]), [0., 1., 1., 0.])

    def test_ne(self):
        assert_close(eval_expr("$close != 5", [3., 5., 5., 7.]), [1., 0., 0., 1.])

    def test_lt(self):
        assert_close(eval_expr("$close < 5", [3., 5., 7.]), [1., 0., 0.])

    def test_le(self):
        assert_close(eval_expr("$close <= 5", [3., 5., 7.]), [1., 1., 0.])

    def test_gt(self):
        assert_close(eval_expr("$close > 5", [3., 5., 7.]), [0., 0., 1.])

    def test_ge(self):
        assert_close(eval_expr("$close >= 5", [3., 5., 7.]), [0., 1., 1.])

    def test_and(self):
        assert_close(eval_expr("1 && 1", [0.]), [1.])

    def test_and_zero(self):
        assert_close(eval_expr("0 && 1", [0.]), [0.])

    def test_or(self):
        assert_close(eval_expr("0 || 0", [0.]), [0.])

    def test_or_one(self):
        assert_close(eval_expr("0 || 1", [0.]), [1.])


class TestUnaryOps:
    """Unary negation and NOT."""

    def test_neg(self):
        assert_close(eval_expr("-$close", [1., -2., 3.]), [-1., 2., -3.])

    def test_not(self):
        assert_close(eval_expr("!$close", [0., 1., 5., -1.]), [1., 0., 0., 0.])


class TestTernary:
    """Ternary and if_ expressions.

    Note: NaN != 0 is True in both Python and Go, so a NaN condition
    takes the true branch.  This is the current Go DSL behaviour.
    """

    def test_ternary_true(self):
        assert_close(eval_expr("$close > 1 ? 10 : 20", [2.]), [10.])

    def test_ternary_false(self):
        assert_close(eval_expr("$close > 1 ? 10 : 20", [0.]), [20.])

    def test_ternary_series(self):
        result = eval_expr("$close > 3 ? 100 : 0", [1., 4., 2., 5., 3.])
        assert_close(result, [0., 100., 0., 100., 0.])

    def test_if_true(self):
        assert_close(eval_expr("if_($close > 1, 10, 20)", [2.]), [10.])

    def test_if_false(self):
        assert_close(eval_expr("if_($close > 1, 10, 20)", [0.]), [20.])

    def test_nested_ternary(self):
        result = eval_expr("$close > 5 ? 100 : ($close > 2 ? 50 : 0)", [1., 3., 6.])
        assert_close(result, [0., 50., 100.])


class TestCombinedExpressions:
    """Multi-operator expressions combining several features."""

    def test_momentum(self):
        """ema(close,20) / ema(close,60) - 1"""
        bars = lin_bars(80)
        result = eval_expr("ema($close, 20) / ema($close, 60) - 1", bars)
        fast = golden_ema(bars, 20)
        slow = golden_ema(bars, 60)
        expected = [math.nan if math.isnan(f) or math.isnan(s) else f / s - 1 for f, s in zip(fast, slow)]
        assert_close(result, expected)

    def test_crossover_signal(self):
        """sma(close,5) > sma(close,20) ? 1 : -1"""
        bars = sine_bars(30)
        result = eval_expr("sma($close, 5) > sma($close, 20) ? 1 : -1", bars)
        fast = golden_sma(bars, 5)
        slow = golden_sma(bars, 20)
        # NaN != 0 → true branch taken (Go parity).  But comparison NaN > x → False.
        # So early bars: both NaN → NaN > NaN = 0.0 → ternary takes false → -1.
        # After warmup of both: fast > slow ? 1 : -1.
        expected = []
        for f, s in zip(fast, slow):
            cond = 1.0 if f > s else 0.0  # NaN > x = 0.0
            expected.append(1.0 if cond != 0 else -1.0)
        assert_close(result, expected)

    def test_volatility_adjusted(self):
        """(close - sma(close,20)) / std(close,20)"""
        bars = lin_bars(30)
        result = eval_expr("($close - sma($close, 20)) / std($close, 20)", bars)
        smas, stds = golden_sma(bars, 20), golden_std(bars, 20)
        expected = [math.nan if math.isnan(s) or math.isnan(d) or d == 0 else (bars[i] - s) / d for i, (s, d) in enumerate(zip(smas, stds))]
        assert_close(result, expected)

    def test_bb_position(self):
        """(close - bb_lower) / (bb_upper - bb_lower)"""
        bars = lin_bars(30)
        result = eval_expr("($close - bb_lower($close, 20, 2)) / (bb_upper($close, 20, 2) - bb_lower($close, 20, 2))", bars)
        smas, stds = golden_sma(bars, 20), golden_std(bars, 20)
        upper = [math.nan if math.isnan(m) or math.isnan(s) else m + 2 * s for m, s in zip(smas, stds)]
        lower = [math.nan if math.isnan(m) or math.isnan(s) else m - 2 * s for m, s in zip(smas, stds)]
        expected = []
        for i in range(len(bars)):
            if math.isnan(upper[i]) or math.isnan(lower[i]) or upper[i] == lower[i]:
                expected.append(math.nan)
            else:
                expected.append((bars[i] - lower[i]) / (upper[i] - lower[i]))
        assert_close(result, expected)

    def test_double_sma_cross(self):
        """sma(close,5) - sma(close,15)"""
        bars = lin_bars(25)
        result = eval_expr("sma($close, 5) - sma($close, 15)", bars)
        fast, slow = golden_sma(bars, 5), golden_sma(bars, 15)
        expected = [math.nan if math.isnan(f) or math.isnan(s) else f - s for f, s in zip(fast, slow)]
        assert_close(result, expected)

    def test_ema_momentum(self):
        """ema(close,10) - ref(ema(close,10), 5)"""
        bars = lin_bars(25)
        result = eval_expr("ema($close, 10) - ref(ema($close, 10), 5)", bars)
        ema_vals = golden_ema(bars, 10)
        ref_vals = golden_ref(ema_vals, 5)
        expected = [math.nan if math.isnan(e) or math.isnan(r) else e - r for e, r in zip(ema_vals, ref_vals)]
        assert_close(result, expected)

    def test_nested_wma(self):
        """wma(wma(close,5), 3)"""
        bars = lin_bars(15)
        result = eval_expr("wma(wma($close, 5), 3)", bars)
        inner = golden_wma(bars, 5)
        expected = golden_wma(inner, 3)
        assert_close(result, expected)

    def test_rsi_condition(self):
        """rsi(14) > 70 ? -1 : (rsi(14) < 30 ? 1 : 0)"""
        bars = lin_bars(30)
        result = eval_expr("rsi(14) > 70 ? -1 : (rsi(14) < 30 ? 1 : 0)", bars)
        rsi_vals = TestOscillators.golden_rsi(bars, 14)
        # Ternary: NaN comparisons → False, but NaN != 0 → True.
        # rsi > 70: NaN > 70 → False (0.0). cond=0.0, take false branch.
        #   rsi < 30: NaN < 30 → False (0.0). cond=0.0, take false branch → 0.
        # So early bars with NaN RSI → 0.0 (not NaN, per Go parity).
        expected = []
        for r in rsi_vals:
            if not math.isnan(r) and r > 70:
                expected.append(-1.0)
            elif not math.isnan(r) and r < 30:
                expected.append(1.0)
            else:
                expected.append(0.0)
        assert_close(result, expected)


class TestEdgeCases:
    """Boundary conditions, NaN propagation, special values."""

    def test_warmup_boundary(self):
        bars = [10., 20., 30., 40., 50., 60.]
        result = eval_expr("sma($close, 5)", bars)
        assert_close(result, [math.nan, math.nan, math.nan, math.nan, 30., 40.])

    def test_nan_propagation(self):
        """NaN in inner → propagated through wrap."""
        bars = [100., 101.]
        result = eval_expr("zscore($close, 3)", bars)
        assert all(math.isnan(r) for r in result)

    def test_constant_series_std(self):
        bars = [5.0] * 10
        result = eval_expr("std($close, 5)", bars)
        for i, r in enumerate(result):
            if i < 4: assert math.isnan(r)
            else: assert r == 0.0

    def test_constant_series_corr(self):
        """corr on constant series → NaN (denominator 0)."""
        bars = const_bars(15, 5.0)
        result = eval_expr("corr($close, $close, 5)", bars)
        for i in range(4): assert math.isnan(result[i])
        for i in range(4, len(bars)): assert math.isnan(result[i])

    def test_div_by_zero_scalar(self):
        assert math.isnan(eval_expr("1 / 0", [0.])[0])

    def test_log_negative(self):
        assert math.isnan(eval_expr("log(-1)", [0.])[0])

    def test_sqrt_negative(self):
        assert math.isnan(eval_expr("sqrt(-1)", [0.])[0])

    def test_large_numbers(self):
        bars = [1e10, 2e10, 3e10, 4e10, 5e10]
        result = eval_expr("sma($close, 3)", bars)
        expected = golden_sma(bars, 3)
        assert_close(result, expected)

    def test_small_numbers(self):
        bars = [1e-8, 2e-8, 3e-8, 4e-8, 5e-8]
        result = eval_expr("sma($close, 3)", bars)
        expected = golden_sma(bars, 3)
        assert_close(result, expected, tol=1e-15)

    def test_negative_prices(self):
        bars = [-5., -3., -1., 1., 3.]
        result = eval_expr("zscore($close, 3)", bars)
        expected = golden_zscore(bars, 3)
        assert_close(result, expected)

    def test_ref_large_lag(self):
        bars = [1., 2., 3.]
        result = eval_expr("ref($close, 10)", bars)
        assert all(math.isnan(r) for r in result)

    def test_rank_all_same(self):
        bars = [5.0] * 10
        result = eval_expr("rank($close, 5)", bars)
        for i, r in enumerate(result):
            if i < 4: assert math.isnan(r)
            else: assert r == 1.0

    def test_rank_strictly_increasing(self):
        bars = [1., 2., 3., 4., 5.]
        result = eval_expr("rank($close, 5)", bars)
        expected = golden_rank(bars, 5)
        assert_close(result, expected)


class TestParseErrors:
    """Parser error handling."""

    def test_unexpected_eof(self):
        with pytest.raises(SyntaxError):
            parse("sma($close, ")

    def test_unknown_field(self):
        with pytest.raises(NameError):
            compile_expr(parse("$nope"), FIELDS)

    def test_unknown_function(self):
        with pytest.raises(NameError):
            compile_expr(parse("nope()"), FIELDS)

    def test_unbalanced_parens(self):
        with pytest.raises(SyntaxError):
            parse("(1 + 2")

    def test_ternary_incomplete(self):
        with pytest.raises(SyntaxError):
            parse("1 ? 2")


class TestReset:
    """Operator reset behaviour."""

    def test_sma_reset(self):
        from alfq_research.factor.dsl.compile import SMA
        s = SMA(3)
        s.eval(100); s.eval(101); s.eval(102)
        assert s.eval(103) == 102.0
        s.reset()
        assert math.isnan(s.eval(200))

    def test_ema_reset(self):
        from alfq_research.factor.dsl.compile import EMA
        e = EMA(5)
        for v in [10] * 10: e.eval(v)
        assert e.eval(10) == 10.0
        e.reset()
        e.eval(20)
        assert math.isnan(e.eval(20))


# ═══════════════════════════════════════════════════════════════════
# Legacy tests (preserved from original)
# ═══════════════════════════════════════════════════════════════════

BARS_SIMPLE = [100.0, 101.0, 102.0, 103.0, 104.0]


def test_sma_legacy():
    result = eval_expr("sma($close, 3)", BARS_SIMPLE)
    expected = [math.nan, math.nan, 101.0, 102.0, 103.0]
    assert_close(result, expected)


def test_ema_legacy():
    result = eval_expr("ema($close, 3)", BARS_SIMPLE[:6])
    assert math.isnan(result[0])
    assert math.isnan(result[1])


def test_parse_simple_legacy():
    node = parse("ema($close, 20) / ema($close, 60) - 1")
    assert node is not None


def test_rsi_warmup_legacy():
    bars = [100.0] * 20
    result = eval_expr("rsi(14)", bars)
    warmup_bars = 15
    for i in range(warmup_bars - 1):
        assert math.isnan(result[i])


def test_binary_ops_legacy():
    result = eval_expr("$close + 10", BARS_SIMPLE)
    for a, e in zip(result, [b + 10 for b in BARS_SIMPLE]):
        assert abs(a - e) < 1e-9
