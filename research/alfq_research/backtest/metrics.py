"""ALFQ Backtest Metrics — Sharpe, Sortino, MaxDD, Calmar, WinRate, ProfitFactor.

All functions accept either list[float] or polars Series.  Annualisation assumes
252 trading days for daily returns; adjust *periods_per_year* for other frequencies.
"""

from __future__ import annotations

import math
from typing import Sequence

import polars as pl


# ── Annualisation helpers ──

_PERIODS_PER_YEAR: dict[str, int] = {
    "1m": 138240,    # 24h × 60m × ~288 trading days / 3?  Actually forex: 24×5×52 / period
    "5m": 27648,
    "15m": 9216,
    "30m": 4608,
    "1h": 2304,
    "4h": 576,
    "1d": 252,
    "1w": 52,
}
# Simplified: use daily-equivalent for most; caller can override.
_DEFAULT_PERIODS = 252


def _to_list(series: Sequence[float] | pl.Series) -> list[float]:
    if isinstance(series, pl.Series):
        return series.to_list()
    return list(series)


# ── Core metrics ──

def annual_return(returns: Sequence[float] | pl.Series, periods: int = _DEFAULT_PERIODS) -> float:
    vals = _to_list(returns)
    if not vals:
        return 0.0
    return sum(vals) / len(vals) * periods


def annual_volatility(returns: Sequence[float] | pl.Series, periods: int = _DEFAULT_PERIODS) -> float:
    vals = _to_list(returns)
    n = len(vals)
    if n < 2:
        return 0.0
    mean = sum(vals) / n
    var = sum((r - mean) ** 2 for r in vals) / (n - 1)
    return math.sqrt(var * periods)


def sharpe_ratio(
    returns: Sequence[float] | pl.Series,
    rf: float = 0.0,
    periods: int = _DEFAULT_PERIODS,
) -> float:
    vals = _to_list(returns)
    n = len(vals)
    if n < 2:
        return 0.0
    excess = [r - rf / periods for r in vals]
    mean = sum(excess) / n
    std = math.sqrt(sum((x - mean) ** 2 for x in excess) / (n - 1))
    return (mean / std) * math.sqrt(periods) if std > 0 else 0.0


def sortino_ratio(
    returns: Sequence[float] | pl.Series,
    rf: float = 0.0,
    periods: int = _DEFAULT_PERIODS,
) -> float:
    vals = _to_list(returns)
    n = len(vals)
    if n < 2:
        return 0.0
    excess = [r - rf / periods for r in vals]
    mean = sum(excess) / n
    downside = [min(0.0, x) for x in excess]
    dd_std = math.sqrt(sum(x ** 2 for x in downside) / (len(downside) - 1))
    return (mean / dd_std) * math.sqrt(periods) if dd_std > 0 else 0.0


def max_drawdown(equity: Sequence[float] | pl.Series) -> float:
    vals = _to_list(equity)
    if not vals:
        return 0.0
    peak = vals[0]
    max_dd = 0.0
    for v in vals:
        if v > peak:
            peak = v
        dd = (peak - v) / peak if peak > 0 else 0.0
        if dd > max_dd:
            max_dd = dd
    return max_dd


def max_dd_duration(equity: Sequence[float] | pl.Series) -> int:
    """Longest number of periods equity spent below its previous peak."""
    vals = _to_list(equity)
    if not vals:
        return 0
    peak = vals[0]
    current_dur = 0
    max_dur = 0
    for v in vals:
        if v >= peak:
            peak = v
            current_dur = 0
        else:
            current_dur += 1
            if current_dur > max_dur:
                max_dur = current_dur
    return max_dur


def calmar_ratio(
    returns: Sequence[float] | pl.Series,
    equity: Sequence[float] | pl.Series,
    periods: int = _DEFAULT_PERIODS,
) -> float:
    ann_ret = annual_return(returns, periods)
    dd = max_drawdown(equity)
    return ann_ret / dd if dd > 0 else 0.0


# ── Trade-level metrics ──

def win_rate(trades_pnl: Sequence[float] | pl.Series) -> float:
    vals = _to_list(trades_pnl)
    if not vals:
        return 0.0
    wins = sum(1 for p in vals if p > 0)
    return wins / len(vals)


def profit_factor(trades_pnl: Sequence[float] | pl.Series) -> float:
    vals = _to_list(trades_pnl)
    gross_profit = sum(p for p in vals if p > 0)
    gross_loss = abs(sum(p for p in vals if p < 0))
    return gross_profit / gross_loss if gross_loss > 0 else float("inf") if gross_profit > 0 else 0.0


def avg_trade(trades_pnl: Sequence[float] | pl.Series) -> float:
    vals = _to_list(trades_pnl)
    return sum(vals) / len(vals) if vals else 0.0


def avg_win(trades_pnl: Sequence[float] | pl.Series) -> float:
    wins = [p for p in _to_list(trades_pnl) if p > 0]
    return sum(wins) / len(wins) if wins else 0.0


def avg_loss(trades_pnl: Sequence[float] | pl.Series) -> float:
    losses = [p for p in _to_list(trades_pnl) if p < 0]
    return sum(losses) / len(losses) if losses else 0.0


def total_return(equity: Sequence[float] | pl.Series, initial_capital: float) -> float:
    vals = _to_list(equity)
    if not vals or initial_capital == 0:
        return 0.0
    return (vals[-1] - initial_capital) / initial_capital


# ── Aggregate metrics dict ──

def compute_all(
    returns: Sequence[float] | pl.Series,
    equity: Sequence[float] | pl.Series,
    trades_pnl: Sequence[float] | pl.Series,
    initial_capital: float,
    periods: int = _DEFAULT_PERIODS,
) -> dict[str, float]:
    """Return a dict of all standard backtest metrics."""
    returns_list = _to_list(returns)
    equity_list = _to_list(equity)
    pnl_list = _to_list(trades_pnl)

    return {
        "total_return": total_return(equity_list, initial_capital),
        "annual_return": annual_return(returns_list, periods),
        "annual_volatility": annual_volatility(returns_list, periods),
        "sharpe_ratio": sharpe_ratio(returns_list, periods=periods),
        "sortino_ratio": sortino_ratio(returns_list, periods=periods),
        "calmar_ratio": calmar_ratio(returns_list, equity_list, periods),
        "max_drawdown": max_drawdown(equity_list),
        "max_dd_duration": max_dd_duration(equity_list),
        "win_rate": win_rate(pnl_list),
        "profit_factor": profit_factor(pnl_list),
        "trade_count": len(pnl_list),
        "avg_trade": avg_trade(pnl_list),
        "avg_win": avg_win(pnl_list),
        "avg_loss": avg_loss(pnl_list),
    }
