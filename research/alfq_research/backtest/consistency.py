"""ALFQ Backtest — Consistency gate between vectorized and event-driven engines.

Gate requirements (per docs/10 §5 and MASTER-ROADMAP §6 RP-3):
  1. Daily PnL Pearson correlation ≥ 0.95
  2. Daily PnL mean absolute deviation (MAD) < 1% of initial capital

Usage:
    from alfq_research.backtest.consistency import consistency_check
    ok, report = consistency_check(result_vec, result_ev, initial_capital)
"""

from __future__ import annotations

import math

import polars as pl

from .vectorized import BacktestResult


def consistency_check(
    vec: BacktestResult,
    ev: BacktestResult,
    initial_capital: float,
) -> tuple[bool, dict]:
    """Run the consistency gate between vectorized and event-driven results.

    Returns
    -------
    passed: bool
        True if both gates are satisfied.
    report: dict
        Detailed metrics: corr, daily_mad, daily_mad_pct, vec_metrics, ev_metrics.
    """
    # Compute daily equity / PnL from both results
    vec_daily = _daily_pnl(vec, initial_capital)
    ev_daily = _daily_pnl(ev, initial_capital)

    # Align by date
    aligned = vec_daily.join(ev_daily, on="date", how="inner", suffix="_ev")
    if len(aligned) < 5:
        return False, {"error": "too few overlapping days", "days": len(aligned)}

    vec_pnl = aligned["daily_pnl"].to_list()
    ev_pnl = aligned["daily_pnl_ev"].to_list()

    # Pearson correlation
    corr = _pearson(vec_pnl, ev_pnl)

    # Mean absolute deviation (MAD) as % of initial capital
    mad = sum(abs(v - e) for v, e in zip(vec_pnl, ev_pnl, strict=True)) / len(vec_pnl)
    mad_pct = mad / initial_capital

    passed = corr >= 0.95 and mad_pct < 0.01

    return passed, {
        "correlation": round(corr, 6),
        "daily_mad": round(mad, 2),
        "daily_mad_pct": round(mad_pct, 6),
        "overlap_days": len(aligned),
        "vec_sharpe": vec.metrics.get("sharpe_ratio", 0.0),
        "ev_sharpe": ev.metrics.get("sharpe_ratio", 0.0),
        "vec_return": vec.metrics.get("total_return", 0.0),
        "ev_return": ev.metrics.get("total_return", 0.0),
        "passed": passed,
    }


def _daily_pnl(result: BacktestResult, initial_capital: float) -> pl.DataFrame:
    """Downsample bar-level equity to daily PnL."""
    df = result.pnl_series
    if "ts" not in df.columns:
        return pl.DataFrame(schema={"date": pl.Utf8, "daily_pnl": pl.Float64})

    # Extract date from ts column (int64 ms or datetime)
    if df["ts"].dtype.is_temporal():
        df = df.with_columns(pl.col("ts").dt.strftime("%Y-%m-%d").alias("date"))
    else:
        df = df.with_columns(
            pl.from_epoch("ts", time_unit="ms")
            .dt.strftime("%Y-%m-%d")
            .alias("date")
        )

    daily = df.group_by("date").agg(
        pl.col("equity").last().alias("equity_eod")
    ).sort("date")

    daily = daily.with_columns(
        pl.col("equity_eod").diff().fill_null(0).alias("daily_pnl")
    )

    return daily.select(["date", "daily_pnl"])


def _pearson(x: list[float], y: list[float]) -> float:
    n = len(x)
    if n < 2:
        return 0.0
    mx = sum(x) / n
    my = sum(y) / n
    num = sum((xi - mx) * (yi - my) for xi, yi in zip(x, y, strict=True))
    dx = math.sqrt(sum((xi - mx) ** 2 for xi in x))
    dy = math.sqrt(sum((yi - my) ** 2 for yi in y))
    if dx == 0 or dy == 0:
        return 0.0
    return num / (dx * dy)
