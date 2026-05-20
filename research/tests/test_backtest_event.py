"""Tests for event-driven backtest engine."""

import polars as pl

from alfq_research.backtest import (
    BacktestConfig,
    BacktestResult,
    EventBacktest,
    BrokerParams,
)


def make_bars(n: int = 100, trend: float = 0.0) -> pl.DataFrame:
    """Generate synthetic OHLCV bars for testing."""
    import numpy as np
    rng = np.random.default_rng(42)
    close = 1.0800 + np.cumsum(rng.normal(trend / n, 0.0005, n))
    open_ = close - rng.normal(0, 0.0001, n)
    high = close + np.abs(rng.normal(0, 0.0002, n))
    low = close - np.abs(rng.normal(0, 0.0002, n))
    volume = rng.uniform(100, 500, n)
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


def test_event_backtest_runs():
    """Event-driven backtest produces valid result."""
    bars = make_bars(60, trend=0.005)
    cfg = BacktestConfig(
        symbols=["EURUSD"],
        factors={"f": "sma($close, 5)"},
        signal_rule="f < $close ? 1 : -1",
        sizing={"type": "fixed_lots", "lots": 0.1},
        broker_params={"EURUSD": BrokerParams()},
    )
    eb = EventBacktest(cfg, bars)
    result = eb.run()
    assert isinstance(result, BacktestResult)
    assert len(result.equity_curve) == 60
    assert "sharpe_ratio" in result.metrics


def test_event_result_caching():
    bars = make_bars(20)
    cfg = BacktestConfig(
        symbols=["EURUSD"],
        signal_rule="1",
        sizing={"type": "fixed_lots", "lots": 0.1},
    )
    eb = EventBacktest(cfg, bars)
    r1 = eb.run()
    r2 = eb.run()
    assert r1 is r2


def test_event_trades_have_correct_columns():
    bars = make_bars(100, trend=0.01)
    cfg = BacktestConfig(
        symbols=["EURUSD"],
        factors={"s": "sma($close, 10)"},
        signal_rule="s < $close ? 1 : -1",
        sizing={"type": "fixed_lots", "lots": 0.1},
    )
    eb = EventBacktest(cfg, bars)
    result = eb.run()
    trades = result.trades
    if len(trades) > 0:
        expected = {"entry_ts", "exit_ts", "symbol", "side", "lots",
                    "entry_price", "exit_price", "pnl", "holding_bars"}
        assert expected.issubset(set(trades.columns))


def test_event_vs_vectorized_both_run():
    """Both engines run without error on same config."""
    from alfq_research.backtest import VectorizedBacktest

    bars = make_bars(80, trend=0.003)
    cfg = BacktestConfig(
        symbols=["EURUSD"],
        factors={"f": "ema($close, 7)"},
        signal_rule="f < $close ? 1 : -1",
        sizing={"type": "fixed_lots", "lots": 0.1},
    )
    v_result = VectorizedBacktest(cfg, bars).run()
    e_result = EventBacktest(cfg, bars).run()
    assert len(v_result.equity_curve) == 80
    assert len(e_result.equity_curve) == 80
