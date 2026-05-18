"""Parity test: Python streaming DSL vs expected values.

This verifies the Python DSL produces results consistent with the Go engine.
Golden test data is generated from known expression + bar sequence pairs.
"""

import math
from alfq_research.factor.dsl.parser import parse
from alfq_research.factor.dsl.compile import compile_expr


# Golden data: bars [100, 101, 102, 103, 104] with sma($close, 3)
# Expected per bar: [NaN, NaN, 101.0, 102.0, 103.0]
BARS = [100.0, 101.0, 102.0, 103.0, 104.0]


def eval_expr(expr: str, bars: list[float]) -> list[float]:
    node = parse(expr)
    fields = {"close": 0, "open": 1, "high": 2, "low": 3, "volume": 4}
    op = compile_expr(node, fields)
    return [op.eval(v) for v in bars]


def assert_close(actual, expected):
    for i, (a, e) in enumerate(zip(actual, expected, strict=True)):
        if math.isnan(e):
            assert math.isnan(a), f"bar {i}: expected NaN, got {a}"
        else:
            assert abs(a - e) < 1e-9, f"bar {i}: expected {e}, got {a}"


def test_sma():
    result = eval_expr("sma($close, 3)", BARS)
    expected = [math.nan, math.nan, 101.0, 102.0, 103.0]
    assert_close(result, expected)


def test_ema():
    result = eval_expr("ema($close, 3)", BARS[:6])
    # First few bars produce NaN during warmup
    assert math.isnan(result[0])
    assert math.isnan(result[1])


def test_parse_simple():
    node = parse("ema($close, 20) / ema($close, 60) - 1")
    assert node is not None


def test_rsi_warmup():
    bars = [100.0] * 20
    result = eval_expr("rsi(14)", bars)
    warmup_bars = 15  # n+1
    for i in range(warmup_bars - 1):
        assert math.isnan(result[i]), f"bar {i}: expected NaN during warmup"


def test_binary_ops():
    result = eval_expr("$close + 10", BARS)
    for _, (a, e) in enumerate(zip(result, [b + 10 for b in BARS], strict=False)):
        assert abs(a - e) < 1e-9
