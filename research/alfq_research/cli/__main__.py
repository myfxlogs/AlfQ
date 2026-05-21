"""ALFQ Research CLI — backtest entry point for Go BacktestService (LP-1).

Usage:
    uv run python -m alfq_research.cli backtest --spec <path> --output json
"""

from __future__ import annotations

import argparse
import json
import os
import sys

import polars as pl

from alfq_research.data.client import DataClient
from alfq_research.backtest import (
    BacktestConfig,
    VectorizedBacktest,
    EventBacktest,
    BrokerParams,
    consistency_check,
)


def cmd_backtest(args: argparse.Namespace) -> None:
    """Run vectorized + event-driven backtest and output consistency gate result."""
    # Load spec
    with open(args.spec) as f:
        spec = json.load(f)

    symbols = spec.get("canonical_symbols", spec.get("symbols", []))
    period = spec.get("period", "1h")
    start = spec.get("start", "2025-01-01")
    end = spec.get("end", "2025-05-21")
    factors = spec.get("factors", {})
    signal_rule = spec.get("signal_rule", "")

    # Fetch bars from ClickHouse
    dc = DataClient(
        ch_host=os.environ.get("ALFQ_CH_HOST", "localhost"),
        ch_port=int(os.environ.get("ALFQ_CH_PORT", "8123")),
        ch_user=os.environ.get("ALFQ_CH_USER", "alfq"),
        ch_password=os.environ.get("ALFQ_CH_PASSWORD", "alfq_dev"),
        ch_db=os.environ.get("ALFQ_CH_DB", "alfq"),
    )

    all_bars = []
    for sym in symbols:
        bars = dc.bars(sym, period=period, start=start, end=end)
        if bars.shape[0] > 0:
            all_bars.append(bars)
    if not all_bars:
        result = {"status": "failed", "error": "no bars found"}
        print(json.dumps(result))
        sys.exit(0)
    bars_df = pl.concat(all_bars)

    cfg = BacktestConfig(
        symbols=symbols,
        factors=factors,
        signal_rule=signal_rule,
        sizing=spec.get("sizing", {"type": "fixed_lots", "lots": 0.1}),
        broker_params={s: BrokerParams() for s in symbols},
    )

    # Run both backtests
    try:
        vec = VectorizedBacktest(cfg, bars_df).run()
        ev = EventBacktest(cfg, bars_df).run()
    except Exception as exc:
        result = {"status": "failed", "error": str(exc)}
        print(json.dumps(result))
        sys.exit(0)

    # Consistency gate
    passed, report = consistency_check(vec, ev, cfg.initial_capital)

    result = {
        "strategy_id": spec.get("name", "unknown"),
        "status": "passed" if passed else "failed",
        "correlation": report.get("correlation", 0),
        "daily_mad_pct": report.get("daily_mad_pct", 0),
        "vec_sharpe": getattr(vec, "metrics", {}).get("sharpe", 0),
        "ev_sharpe": getattr(ev, "metrics", {}).get("sharpe", 0),
        "vec_return": getattr(vec, "metrics", {}).get("total_return", 0),
        "ev_return": getattr(ev, "metrics", {}).get("total_return", 0),
        "overlap_days": 0,
    }
    print(json.dumps(result))


def main() -> None:
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="command")

    bt = sub.add_parser("backtest")
    bt.add_argument("--spec", required=True, help="Path to strategy spec JSON")
    bt.add_argument("--output", default="json")

    args = parser.parse_args()
    if args.command == "backtest":
        cmd_backtest(args)
    else:
        parser.print_help()


if __name__ == "__main__":
    main()
