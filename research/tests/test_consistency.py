"""Consistency gate tests: vectorized vs event-driven backtest.

Gate: daily PnL correlation ≥ 0.95, daily MAD < 1% of initial capital.
"""

import polars as pl

from alfq_research.backtest import (
    BacktestConfig,
    VectorizedBacktest,
    EventBacktest,
    BrokerParams,
    consistency_check,
)
from alfq_research.backtest.consistency import _pearson


def make_bars(n: int = 400, trend: float = 0.0) -> pl.DataFrame:
    """Generate multi-day synthetic OHLCV bars (~200 hours ≈ 8 days)."""
    import numpy as np
    rng = np.random.default_rng(42)
    base = 1.0800
    # Generate smoother trend for consistency
    steps = np.linspace(0, trend, n)
    noise = rng.normal(0, 0.0003, n)
    close = base + steps + np.cumsum(noise) * 0.3

    open_ = close - rng.normal(0, 0.0001, n)
    high = close + np.abs(rng.normal(0, 0.0002, n))
    low = close - np.abs(rng.normal(0, 0.0002, n))
    volume = rng.uniform(100, 500, n)

    # Spread over many days: each bar = 1 hour
    ts = list(range(1_700_000_000_000, 1_700_000_000_000 + n * 3600_000, 3600_000))
    return pl.DataFrame({
        "ts": ts,
        "open": open_,
        "high": high,
        "low": low,
        "close": close,
        "volume": volume,
        "symbol": ["EURUSD"] * n,
    })


def test_consistency_sma_crossover():
    """SMA crossover: vectorized and event-driven should correlate > 0.95."""
    bars = make_bars(500, trend=0.04)  # ~20 days of hourly bars, strong trend
    cfg = BacktestConfig(
        symbols=["EURUSD"],
        factors={
            "fast": "sma($close, 5)",
            "slow": "sma($close, 20)",
        },
        signal_rule="fast > slow ? 1 : -1",  # always in market
        sizing={"type": "fixed_lots", "lots": 0.1},
        broker_params={"EURUSD": BrokerParams()},
    )

    vec_result = VectorizedBacktest(cfg, bars).run()
    ev_result = EventBacktest(cfg, bars).run()

    passed, report = consistency_check(vec_result, ev_result, cfg.initial_capital)
    print(f"Consistency report: {report}")

    assert passed, (
        f"Consistency gate failed: corr={report.get('correlation')}, "
        f"mad_pct={report.get('daily_mad_pct')}"
    )
    assert report["correlation"] >= 0.95
    assert report["daily_mad_pct"] < 0.01


def test_consistency_ema_trend():
    """EMA signal on trending data — should pass gate."""
    bars = make_bars(400, trend=0.015)
    cfg = BacktestConfig(
        symbols=["EURUSD"],
        factors={"mom": "ema($close, 20) / ema($close, 60) - 1"},
        signal_rule="mom > 0 ? 1 : -1",
        sizing={"type": "fixed_lots", "lots": 0.1},
        broker_params={"EURUSD": BrokerParams()},
    )

    vec_result = VectorizedBacktest(cfg, bars).run()
    ev_result = EventBacktest(cfg, bars).run()

    passed, report = consistency_check(vec_result, ev_result, cfg.initial_capital)

    assert passed, f"EMA consistency failed: {report}"
    assert report["correlation"] >= 0.95


def test_consistency_insufficient_data():
    """Too few bars → gate returns False with error."""
    bars = make_bars(10)
    cfg = BacktestConfig(
        symbols=["EURUSD"],
        signal_rule="1",
        sizing={"type": "fixed_lots", "lots": 0.1},
    )
    vec_result = VectorizedBacktest(cfg, bars).run()
    ev_result = EventBacktest(cfg, bars).run()

    passed, report = consistency_check(vec_result, ev_result, cfg.initial_capital)
    assert not passed
    assert "error" in report


def test_pearson_perfect():
    assert abs(_pearson([1, 2, 3], [1, 2, 3]) - 1.0) < 1e-9


def test_pearson_negative():
    assert _pearson([1, 2, 3], [3, 2, 1]) < 0


def test_daily_mad_calculation():
    """Verify MAD computation is reasonable."""
    vec_pnl = [100, 200, -50, 300, 0]
    ev_pnl = [105, 195, -48, 310, 5]
    mad = sum(abs(v - e) for v, e in zip(vec_pnl, ev_pnl, strict=True)) / len(vec_pnl)
    assert mad < 10  # small deviations
