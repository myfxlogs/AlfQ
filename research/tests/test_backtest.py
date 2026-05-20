"""Tests for ALFQ vectorized backtest engine."""

import math

import polars as pl
import pytest

from alfq_research.backtest import (
    BacktestConfig,
    BacktestResult,
    VectorizedBacktest,
    BrokerParams,
    FeeConfig,
)
from alfq_research.backtest.metrics import (
    sharpe_ratio,
    sortino_ratio,
    max_drawdown,
    max_dd_duration,
    calmar_ratio,
    win_rate,
    profit_factor,
    compute_all,
)


# ═══════════════════════════════════════════════════════════════════
# Helpers
# ═══════════════════════════════════════════════════════════════════

def make_bars(n: int = 100, trend: float = 0.0) -> pl.DataFrame:
    """Generate synthetic OHLCV bars for testing."""
    import numpy as np
    rng = np.random.default_rng(42)
    close = 1.0800 + np.cumsum(rng.normal(trend / n, 0.0005, n))
    high = close + np.abs(rng.normal(0, 0.0002, n))
    low = close - np.abs(rng.normal(0, 0.0002, n))
    open_ = close - rng.normal(0, 0.0001, n)
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


# ═══════════════════════════════════════════════════════════════════
# Config tests
# ═══════════════════════════════════════════════════════════════════

def test_config_defaults():
    cfg = BacktestConfig()
    assert cfg.initial_capital == 100_000.0
    assert cfg.symbols == []
    assert cfg.factors == {}


def test_config_with_factors():
    cfg = BacktestConfig(
        symbols=["EURUSD"],
        factors={"mom": "ema($close, 20) / ema($close, 60) - 1"},
        signal_rule="mom > 0 ? 1 : -1",
        sizing={"type": "fixed_lots", "lots": 0.1},
    )
    assert len(cfg.factors) == 1
    assert cfg.signal_rule.startswith("mom")
    assert cfg.sizing["lots"] == 0.1


# ═══════════════════════════════════════════════════════════════════
# Backtest engine tests
# ═══════════════════════════════════════════════════════════════════

def test_sma_crossover_basic():
    """SMA(5) > SMA(15) crossover strategy on trending bars."""
    bars = make_bars(100, trend=0.01)  # strong uptrend
    cfg = BacktestConfig(
        symbols=["EURUSD"],
        factors={
            "fast": "sma($close, 5)",
            "slow": "sma($close, 15)",
        },
        signal_rule="fast > slow ? 1 : (fast < slow ? -1 : 0)",
        sizing={"type": "fixed_lots", "lots": 0.1},
        broker_params={"EURUSD": BrokerParams(contract_size=100_000)},
    )
    bt = VectorizedBacktest(cfg, bars)
    result = bt.run()

    assert isinstance(result, BacktestResult)
    assert len(result.equity_curve) == 100
    # With strong trend, at least some trades should occur
    assert "sharpe_ratio" in result.metrics
    assert "max_drawdown" in result.metrics
    assert "win_rate" in result.metrics


def test_empty_factors():
    """No factors → constant position test."""
    bars = make_bars(20)
    cfg = BacktestConfig(
        symbols=["EURUSD"],
        signal_rule="1",  # always long
        sizing={"type": "fixed_lots", "lots": 0.1},
        broker_params={"EURUSD": BrokerParams()},
    )
    bt = VectorizedBacktest(cfg, bars)
    result = bt.run()
    assert result.equity_curve[0] == cfg.initial_capital


def test_result_caching():
    """Second run returns cached result."""
    bars = make_bars(30)
    cfg = BacktestConfig(
        symbols=["EURUSD"],
        factors={"s": "sma($close, 5)"},
        signal_rule="s > $close ? 1 : -1",
        sizing={"type": "fixed_lots", "lots": 0.1},
    )
    bt = VectorizedBacktest(cfg, bars)
    r1 = bt.run()
    r2 = bt.run()
    assert r1 is r2


def test_trade_extraction():
    """Verify trades DataFrame has correct columns."""
    bars = make_bars(50, trend=0.0005)
    cfg = BacktestConfig(
        symbols=["EURUSD"],
        factors={"s": "sma($close, 10)"},
        signal_rule="s < $close ? 1 : -1",
        sizing={"type": "fixed_lots", "lots": 0.1},
    )
    bt = VectorizedBacktest(cfg, bars)
    result = bt.run()
    trades = result.trades
    if len(trades) > 0:
        expected_cols = {"entry_ts", "exit_ts", "symbol", "side", "lots",
                         "entry_price", "exit_price", "pnl", "holding_bars"}
        assert expected_cols.issubset(set(trades.columns))


# ═══════════════════════════════════════════════════════════════════
# Metrics tests
# ═══════════════════════════════════════════════════════════════════

def test_sharpe_positive():
    returns = [0.001 if i % 2 == 0 else 0.002 for i in range(252)]
    sr = sharpe_ratio(returns)
    assert sr > 0


def test_sharpe_zero():
    returns = [0.0] * 252
    sr = sharpe_ratio(returns)
    assert sr == 0.0


def test_max_drawdown_simple():
    equity = [100, 110, 90, 95, 105]
    dd = max_drawdown(equity)
    # Peak 110, trough 90 → dd = (110-90)/110 = 0.1818...
    assert abs(dd - 20 / 110) < 1e-9


def test_max_dd_duration():
    equity = [100, 110, 105, 115]
    dur = max_dd_duration(equity)
    assert dur == 1


def test_win_rate():
    pnl = [100, -50, 200, -30, 50]
    wr = win_rate(pnl)
    assert abs(wr - 0.6) < 1e-9


def test_profit_factor():
    pnl = [100, -50, 200, -30, 50]
    pf = profit_factor(pnl)
    # Gross profit = 100+200+50=350, Gross loss = 80, PF = 350/80 = 4.375
    assert abs(pf - 350 / 80) < 1e-9


def test_sortino():
    returns = [0.001, 0.002, -0.001, 0.003, -0.002] * 50
    sr = sortino_ratio(returns, periods=252)
    assert sr > 0


def test_calmar():
    returns = [0.001] * 252
    equity = [100000] + [100000 + i * 100 for i in range(1, 253)]
    equity[50] = 90000  # create a drawdown
    cr = calmar_ratio(returns, equity)
    assert cr > 0


def test_compute_all():
    returns = [0.001, -0.0005, 0.002] * 84
    equity = [100000.0]
    for r in returns:
        equity.append(equity[-1] * (1 + r))
    pnl = [r * 100000 for r in returns]
    metrics = compute_all(returns=returns, equity=equity, trades_pnl=pnl, initial_capital=100000)
    assert metrics["trade_count"] == len(pnl)
    assert "sharpe_ratio" in metrics
    assert "max_drawdown" in metrics
    assert "win_rate" in metrics
    assert "profit_factor" in metrics


# ═══════════════════════════════════════════════════════════════════
# Broker sim tests
# ═══════════════════════════════════════════════════════════════════

def test_compute_commission():
    from alfq_research.backtest.broker_sim import compute_commission
    fee = FeeConfig(commission_per_lot=7.0)
    assert compute_commission(1.0, fee) == 7.0
    assert abs(compute_commission(0.1, fee) - 0.7) < 1e-9


def test_compute_trade_pnl_long_win():
    from alfq_research.backtest.broker_sim import compute_trade_pnl
    bp = BrokerParams(contract_size=100_000)
    pnl = compute_trade_pnl(1.0800, 1.0850, 1.0, "buy", bp)
    # Long: price diff = +0.0050, gross = 500. Minus commission ~7, minus small slippage
    assert pnl > 0
    assert pnl < 500


def test_compute_trade_pnl_short_win():
    from alfq_research.backtest.broker_sim import compute_trade_pnl
    bp = BrokerParams(contract_size=100_000)
    pnl = compute_trade_pnl(1.0850, 1.0800, 1.0, "sell", bp)
    assert pnl > 0
