"""ALFQ Backtest — Vectorized engine (Polars-based).

Flow:
  bars → factor computation → signal generation → position sizing → PnL → metrics

All per-bar logic uses the DSL streaming operators for consistency with Go.
Metrics are computed in vectorized fashion via the metrics module.
"""

from __future__ import annotations

import math
from dataclasses import dataclass, field
from typing import Any

import polars as pl

from alfq_research.factor.dsl.parser import parse
from alfq_research.factor.dsl.compile import compile_expr

from .broker_sim import BrokerParams, FeeConfig, compute_trade_pnl  # noqa: F401
from .metrics import compute_all


# ═══════════════════════════════════════════════════════════════════════
# Configuration
# ═══════════════════════════════════════════════════════════════════════

@dataclass
class BacktestConfig:
    """Input configuration for a vectorized backtest."""

    symbols: list[str] = field(default_factory=list)
    period: str = "1h"
    start: str = ""
    end: str = ""
    initial_capital: float = 100_000.0
    factors: dict[str, str] = field(default_factory=dict)
    signal_rule: str = ""                # DSL expression: >0 → long, <0 → short
    sizing: dict[str, Any] = field(default_factory=dict)  # e.g. {"type": "fixed_lots", "lots": 0.1}
    fees: FeeConfig = field(default_factory=FeeConfig)
    broker_params: dict[str, BrokerParams] = field(default_factory=dict)
    broker_id: str = ""                  # for fetching broker params from PG


@dataclass
class BacktestResult:
    """Output of a backtest run."""

    pnl_series: pl.DataFrame = field(default_factory=pl.DataFrame)
    trades: pl.DataFrame = field(default_factory=pl.DataFrame)
    equity_curve: list[float] = field(default_factory=list)
    metrics: dict[str, float] = field(default_factory=dict)
    config: BacktestConfig | None = None


# ═══════════════════════════════════════════════════════════════════════
# Engine
# ═══════════════════════════════════════════════════════════════════════

# Field names used by DSL field references
_FIELDS = {"close": 0, "open": 1, "high": 2, "low": 3, "volume": 4}


class VectorizedBacktest:
    """Polars-based vectorized backtest engine.

    Parameters
    ----------
    config: BacktestConfig
        Strategy and execution parameters.
    bars: polars.DataFrame
        OHLCV data with columns: ts, open, high, low, close, volume, symbol.
        Must be sorted by ts ascending.
    """

    def __init__(self, config: BacktestConfig, bars: pl.DataFrame):
        self.config = config
        self.bars = bars
        self._results: BacktestResult | None = None

    # ── Run ──

    def run(self) -> BacktestResult:
        """Execute the backtest and return results."""
        if self._results is not None:
            return self._results

        # Compile factor expressions
        compiled_factors = self._compile_factors()

        # Evaluate bar-by-bar
        records = self._evaluate(compiled_factors)

        # Build result DataFrames
        result_df = pl.DataFrame(records)
        trades_df = self._extract_trades(result_df)
        pnl_df = result_df.select(["ts", "equity"]).with_columns(
            (pl.col("equity").diff().fill_null(0)).alias("pnl")
        )

        # Compute metrics
        returns = pnl_df["pnl"].to_list()
        equity = result_df["equity"].to_list()
        trade_pnl = trades_df["pnl"].to_list() if len(trades_df) > 0 else [0.0]
        metrics = compute_all(
            returns=returns,
            equity=equity,
            trades_pnl=trade_pnl,
            initial_capital=self.config.initial_capital,
        )

        self._results = BacktestResult(
            pnl_series=pnl_df,
            trades=trades_df,
            equity_curve=equity,
            metrics=metrics,
            config=self.config,
        )
        return self._results

    # ── Factor compilation ──

    def _compile_factors(self) -> dict[str, object]:
        """Compile all factor DSL expressions into evaluable operators."""
        compiled: dict[str, object] = {}
        for name, expr in self.config.factors.items():
            node = parse(expr)
            op = compile_expr(node, _FIELDS)
            compiled[name] = op
        return compiled

    # ── Bar-by-bar evaluation ──

    def _evaluate(self, factors: dict[str, object]) -> list[dict[str, object]]:
        """Iterate through bars, compute factors/signals/positions/PnL."""
        records: list[dict[str, object]] = []
        equity = self.config.initial_capital
        position = 0.0         # current position in lots (positive=long, negative=short)
        entry_price = 0.0
        lots_per_trade = self._get_lots()
        symbol = self.config.symbols[0] if self.config.symbols else "UNKNOWN"
        bp = self.config.broker_params.get(symbol, BrokerParams())

        bar_count = len(self.bars)
        for i in range(bar_count):
            row = self.bars.row(i, named=True)
            close_price = float(row["close"])

            # Evaluate factors for this bar
            factor_vals: dict[str, float] = {}
            for name, op in factors.items():
                val = op.eval(close_price)
                factor_vals[name] = val

            # Compute signal
            signal_val = self._compute_signal(factor_vals)

            # Position management
            trade_pnl = 0.0

            if signal_val > 0 and position <= 0:
                # Enter long or reverse
                if position < 0:
                    # Close short
                    trade_pnl = compute_trade_pnl(
                        entry_price, close_price, abs(position), "sell", bp,
                    )
                    equity += trade_pnl
                position = lots_per_trade
                entry_price = close_price
            elif signal_val < 0 and position >= 0:
                # Enter short or reverse
                if position > 0:
                    # Close long
                    trade_pnl = compute_trade_pnl(
                        entry_price, close_price, abs(position), "buy", bp,
                    )
                    equity += trade_pnl
                position = -lots_per_trade
                entry_price = close_price
            elif signal_val == 0 and position != 0:
                # Close position
                side = "buy" if position > 0 else "sell"
                trade_pnl = compute_trade_pnl(
                    entry_price, close_price, abs(position), side, bp,
                )
                equity += trade_pnl
                position = 0.0
            else:
                # Hold — no change in position
                pass

            records.append({
                "ts": row["ts"],
                "close": close_price,
                "signal": signal_val,
                "position": position,
                "equity": equity,
                "trade_pnl": trade_pnl,
                **factor_vals,
            })

        return records

    def _compute_signal(self, factor_vals: dict[str, float]) -> float:
        """Evaluate the signal rule DSL expression with current factor values."""
        rule = self.config.signal_rule.strip()
        if not rule:
            return 0.0

        # Simple signal rules: >0 → long, <0 → short
        # For now, support direct factor reference or simple comparisons
        try:
            # Try parsing as DSL expression
            node = parse(rule)
            # Map factor names as special fields
            factor_ops = {}
            for name in self.config.factors:
                val = factor_vals.get(name, math.nan)
                from alfq_research.factor.dsl.compile import _Const
                factor_ops[name] = _Const(val)
            op = compile_expr(node, _FIELDS, factors=factor_ops)
            return op.eval(0.0)  # dummy value
        except Exception:
            # Fallback: direct factor value
            if rule in factor_vals:
                return factor_vals[rule]
            return 0.0

    # ── Trade extraction ──

    def _extract_trades(self, result_df: pl.DataFrame) -> pl.DataFrame:
        """Extract individual trades from position changes."""
        trades = []
        position_col = result_df["position"].to_list()
        equity_col = result_df["equity"].to_list()
        close_col = result_df["close"].to_list()
        ts_col = result_df["ts"].to_list()

        in_trade = False
        entry_idx = 0
        entry_price = 0.0
        entry_side = ""
        entry_lots = 0.0
        prev_equity = self.config.initial_capital

        for i in range(len(position_col)):
            pos = position_col[i]
            eq = equity_col[i]
            px = close_col[i]
            ts = ts_col[i]

            if not in_trade and pos != 0:
                in_trade = True
                entry_idx = i
                entry_price = px
                entry_side = "long" if pos > 0 else "short"
                entry_lots = abs(pos)
                prev_equity = eq
            elif in_trade and pos == 0:
                in_trade = False
                trade_pnl = eq - prev_equity
                trades.append({
                    "entry_ts": ts_col[entry_idx],
                    "exit_ts": ts,
                    "symbol": self.config.symbols[0] if self.config.symbols else "",
                    "side": entry_side,
                    "lots": entry_lots,
                    "entry_price": entry_price,
                    "exit_price": px,
                    "pnl": trade_pnl,
                    "holding_bars": i - entry_idx,
                })

        return pl.DataFrame(trades) if trades else pl.DataFrame(
            schema={"entry_ts": pl.Utf8, "exit_ts": pl.Utf8, "symbol": pl.Utf8,
                    "side": pl.Utf8, "lots": pl.Float64, "entry_price": pl.Float64,
                    "exit_price": pl.Float64, "pnl": pl.Float64, "holding_bars": pl.Int64}
        )

    def _get_lots(self) -> float:
        sizing = self.config.sizing
        if sizing.get("type") == "fixed_lots":
            return float(sizing.get("lots", 0.1))
        if sizing.get("type") == "pct_equity":
            pct = float(sizing.get("pct", 2.0)) / 100.0
            risk_amount = self.config.initial_capital * pct
            symbol = self.config.symbols[0] if self.config.symbols else ""
            bp = self.config.broker_params.get(symbol, BrokerParams())
            lot_value = bp.contract_size * (bp.tick_value / bp.tick_size * bp.point) if bp.tick_size > 0 else 100_000
            return max(bp.min_lot, round(risk_amount / lot_value / bp.lot_step) * bp.lot_step)
        return 0.1  # default
