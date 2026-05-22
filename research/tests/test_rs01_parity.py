"""RS01 DSL parity test: Go/Python SMA/EMA/RSI cross-verification.

Loads Go-computed golden values and verifies Python reference implementation
produces bit-identical results (abs diff < 1e-9).

Acceptance: All non-NaN values match between Go and Python.
"""
from __future__ import annotations

import json
import math
import os

import pytest


def load_json(path: str) -> dict:
    with open(path) as f:
        return json.load(f)


def compute_sma(closes: list[float], window: int) -> list[float | None]:
    result: list[float | None] = []
    for i in range(len(closes)):
        if i < window - 1:
            result.append(None)
        else:
            result.append(sum(closes[i - window + 1 : i + 1]) / window)
    return result


def compute_ema(closes: list[float], window: int) -> list[float | None]:
    """Go-compatible EMA: seed=first value, smooth from bar 0, NaN until warmup."""
    alpha = 2.0 / (window + 1)
    result: list[float | None] = []
    ema = 0.0
    count = 0
    for v in closes:
        if count == 0:
            ema = v
        else:
            ema = alpha * v + (1 - alpha) * ema
        count += 1
        result.append(None if count < window else ema)
    return result


def compute_rsi(closes: list[float], window: int = 14) -> list[float | None]:
    if len(closes) < window + 1:
        return [None] * len(closes)

    result: list[float | None] = [None] * window

    gains: list[float] = []
    losses: list[float] = []
    for i in range(1, window + 1):
        diff = closes[i] - closes[i - 1]
        gains.append(max(diff, 0))
        losses.append(max(-diff, 0))

    avg_gain = sum(gains) / window
    avg_loss = sum(losses) / window
    result.append(100.0 if avg_loss == 0 else 100.0 - 100.0 / (1.0 + avg_gain / avg_loss))

    for i in range(window + 1, len(closes)):
        diff = closes[i] - closes[i - 1]
        gain = max(diff, 0)
        loss = max(-diff, 0)
        avg_gain = (avg_gain * (window - 1) + gain) / window
        avg_loss = (avg_loss * (window - 1) + loss) / window
        result.append(100.0 if avg_loss == 0 else 100.0 - 100.0 / (1.0 + avg_gain / avg_loss))

    return result


FIXTURES = os.path.join(os.path.dirname(__file__), "fixtures")


class TestRS01DSLParity:
    """RS01: Go/Python DSL bit-identical parity."""

    @pytest.fixture(scope="class")
    def golden_bars(self) -> list[dict]:
        path = os.path.join(FIXTURES, "golden_bars.json")
        if not os.path.exists(path):
            pytest.skip("golden_bars.json not found")
        return load_json(path)

    @pytest.fixture(scope="class")
    def go_golden(self) -> dict:
        path = os.path.join(FIXTURES, "go_golden_values.json")
        if not os.path.exists(path):
            pytest.skip("go_golden_values.json not found")
        return load_json(path)

    @pytest.fixture(scope="class")
    def closes(self, golden_bars: list[dict]) -> list[float]:
        return [b["close"] for b in golden_bars]

    def test_sma20_bit_identical(self, closes: list[float], go_golden: dict):
        py_vals = compute_sma(closes, 20)
        go_vals = go_golden["sma20"]
        assert len(py_vals) == len(go_vals), "length mismatch"

        mismatches = 0
        for i, (pv, gv) in enumerate(zip(py_vals, go_vals)):
            if pv is None and gv is None:
                continue
            if pv is None or gv is None:
                mismatches += 1
                continue
            if abs(pv - gv) >= 1e-9:
                mismatches += 1
                if mismatches <= 3:
                    print(f"  SMA20[{i}]: py={pv:.10f} go={gv:.10f} diff={abs(pv-gv):.2e}")

        assert mismatches == 0, f"SMA20: {mismatches} mismatches out of {len(py_vals)} values"

    def test_ema60_bit_identical(self, closes: list[float], go_golden: dict):
        py_vals = compute_ema(closes, 60)
        go_vals = go_golden["ema60"]
        assert len(py_vals) == len(go_vals), "length mismatch"

        mismatches = 0
        for i, (pv, gv) in enumerate(zip(py_vals, go_vals)):
            if pv is None and gv is None:
                continue
            if pv is None or gv is None:
                mismatches += 1
                continue
            if abs(pv - gv) >= 1e-9:
                mismatches += 1
                if mismatches <= 3:
                    print(f"  EMA60[{i}]: py={pv:.10f} go={gv:.10f} diff={abs(pv-gv):.2e}")

        assert mismatches == 0, f"EMA60: {mismatches} mismatches out of {len(py_vals)} values"

    def test_rsi14_bit_identical(self, closes: list[float], go_golden: dict):
        py_vals = compute_rsi(closes, 14)
        go_vals = go_golden["rsi14"]
        assert len(py_vals) == len(go_vals), "length mismatch"

        mismatches = 0
        for i, (pv, gv) in enumerate(zip(py_vals, go_vals)):
            if pv is None and gv is None:
                continue
            if pv is None or gv is None:
                mismatches += 1
                continue
            if abs(pv - gv) >= 1e-9:
                mismatches += 1
                if mismatches <= 3:
                    print(f"  RSI14[{i}]: py={pv:.10f} go={gv:.10f} diff={abs(pv-gv):.2e}")

        assert mismatches == 0, f"RSI14: {mismatches} mismatches out of {len(py_vals)} values"

    def test_all_null_positions_match(self, closes: list[float], go_golden: dict):
        """Verify warmup null positions are identical between Go and Python."""
        py_sma = compute_sma(closes, 20)
        py_ema = compute_ema(closes, 60)
        py_rsi = compute_rsi(closes, 14)
        go_sma = go_golden["sma20"]
        go_ema = go_golden["ema60"]
        go_rsi = go_golden["rsi14"]

        for name, py_vals, go_vals in [
            ("SMA20", py_sma, go_sma),
            ("EMA60", py_ema, go_ema),
            ("RSI14", py_rsi, go_rsi),
        ]:
            null_positions_py = {i for i, v in enumerate(py_vals) if v is None}
            null_positions_go = {i for i, v in enumerate(go_vals) if v is None}
            diff = null_positions_py ^ null_positions_go
            assert len(diff) == 0, f"{name}: null position mismatch at indices {sorted(diff)}"
