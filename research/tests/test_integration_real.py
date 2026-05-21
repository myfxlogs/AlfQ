"""Phase C integration tests — real ClickHouse data.

Runs RP-2 (vectorized backtest) and RP-3 (consistency gate) on live CH bars.
Requires ALFQ_CH_* env vars pointing to a running ClickHouse.
"""

import os
import polars as pl
import pytest

from alfq_research.data.client import DataClient
from alfq_research.backtest import (
    BacktestConfig,
    VectorizedBacktest,
    EventBacktest,
    BrokerParams,
    consistency_check,
)


# ── helpers ──────────────────────────────────────────────────────────

def _client() -> DataClient:
    return DataClient(
        ch_host=os.environ.get("ALFQ_CH_HOST", "localhost"),
        ch_port=int(os.environ.get("ALFQ_CH_PORT", "8123")),
        ch_user=os.environ.get("ALFQ_CH_USER", "alfq"),
        ch_password=os.environ.get("ALFQ_CH_PASSWORD", "alfq_dev"),
        ch_db=os.environ.get("ALFQ_CH_DB", "alfq"),
    )


def _fetch_bars(symbol: str, period: str, start: str, end: str) -> pl.DataFrame:
    """Fetch bars from real CH and return a polars DataFrame with required columns."""
    dc = _client()
    df = dc.bars(symbol, period=period, start=start, end=end)
    assert df.shape[0] > 0, f"no bars for {symbol} {period} {start}→{end}"
    # Ensure required columns exist
    for col in ("ts", "open", "high", "low", "close", "volume", "symbol"):
        assert col in df.columns, f"column {col} missing from bars"
    return df


# ── RP-2: vectorized backtest on real data ───────────────────────────

def test_real_bars_sma_crossover():
    """SMA crossover on real 1h EURUSD bars — should produce trades."""
    bars = _fetch_bars("EURUSD", "1h", "2025-05-01", "2025-05-21")
    assert bars.shape[0] >= 100, f"only {bars.shape[0]} bars, need ≥100 for meaningful test"

    cfg = BacktestConfig(
        symbols=["EURUSD"],
        factors={
            "fast": "sma($close, 5)",
            "slow": "sma($close, 20)",
        },
        signal_rule="fast > slow ? 1 : -1",
        sizing={"type": "fixed_lots", "lots": 0.1},
        broker_params={"EURUSD": BrokerParams()},
    )

    result = VectorizedBacktest(cfg, bars).run()
    assert result is not None
    assert hasattr(result, "trades")
    assert hasattr(result, "equity_curve")
    print(f"RP-2 real: {len(result.trades)} trades, sharpe={result.metrics.get('sharpe', 'N/A')}")


def test_real_bars_multi_period():
    """Multiple periods from real CH data for EURUSD."""
    bars = _fetch_bars("EURUSD", "1h", "2025-05-01", "2025-05-21")
    assert bars.shape[0] >= 300, f"1h: only {bars.shape[0]} bars"

    bars = _fetch_bars("EURUSD", "30m", "2025-05-01", "2025-05-20")
    assert bars.shape[0] >= 100, f"30m: only {bars.shape[0]} bars"

    print(f"RP-2 multi-period: 1h={bars.shape[0]} ok")


# ── RP-3: consistency gate on real data ──────────────────────────────

@pytest.mark.slow
def test_real_consistency_sma():
    """Consistency gate (corr ≥ 0.95) on real 1h EURUSD bars."""
    bars = _fetch_bars("EURUSD", "1h", "2025-05-01", "2025-05-21")
    assert bars.shape[0] >= 200, f"only {bars.shape[0]} bars, need ≥200 for consistency"

    cfg = BacktestConfig(
        symbols=["EURUSD"],
        factors={
            "fast": "sma($close, 5)",
            "slow": "sma($close, 20)",
        },
        signal_rule="fast > slow ? 1 : -1",
        sizing={"type": "fixed_lots", "lots": 0.1},
        broker_params={"EURUSD": BrokerParams()},
    )

    vec = VectorizedBacktest(cfg, bars).run()
    ev = EventBacktest(cfg, bars).run()

    passed, report = consistency_check(vec, ev, cfg.initial_capital)
    print(f"RP-3 consistency: passed={passed} corr={report.get('correlation', '?')} "
          f"mad_pct={report.get('daily_mad_pct', '?')}")

    assert passed, f"gate failed: {report}"
    assert report["correlation"] >= 0.95, f"corr={report['correlation']}"
    assert report["daily_mad_pct"] < 0.01, f"mad={report['daily_mad_pct']}"


# ── DataClient schema validation ─────────────────────────────────────

def test_bars_schema():
    """Verify DataClient returns expected columns from real CH."""
    bars = _fetch_bars("EURUSD", "1h", "2025-05-20", "2025-05-21")
    expected = {"symbol", "ts", "open", "high", "low", "close", "volume"}
    missing = expected - set(bars.columns)
    assert not missing, f"missing columns: {missing}"
    assert bars["ts"].dtype.is_temporal()
    assert bars["close"].dtype.is_numeric()
