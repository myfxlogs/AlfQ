"""ALFQ Backtest — Event-driven engine (bar-close stepping with next-bar-open execution).

Key difference from vectorized:
- Signal at bar N is executed at bar N+1 open ± slippage (1-bar delay)
- Each bar is processed as a discrete event
- Uses DSL streaming operators for factor evaluation (per-bar state)

This models real-world execution latency: you see the close, decide, and get filled
at the next bar's open.
"""

from __future__ import annotations

import math

import polars as pl

from alfq_research.factor.dsl.parser import parse
from alfq_research.factor.dsl.compile import compile_expr

from .vectorized import BacktestConfig, BacktestResult
from .broker_sim import BrokerParams, FeeConfig, compute_trade_pnl
from .metrics import compute_all

# Field mapping for DSL (single-float streaming)
_FIELDS = {"close": 0, "open": 1, "high": 2, "low": 3, "volume": 4}


class EventBacktest:
    """Event-driven backtest engine with bar-close evaluation and next-bar-open execution.

    Parameters
    ----------
    config: BacktestConfig
    bars: polars.DataFrame
        OHLCV data (ts, open, high, low, close, volume, symbol), sorted by ts.
    """

    def __init__(self, config: BacktestConfig, bars: pl.DataFrame):
        self.config = config
        self.bars = bars
        self._results: BacktestResult | None = None

    # ── Run ──

    def run(self) -> BacktestResult:
        if self._results is not None:
            return self._results

        compiled = self._compile_factors()
        records = self._step_bars(compiled)
        result_df = pl.DataFrame(records)
        trades_df = self._extract_trades(records)

        # Daily PnL from equity changes
        returns = self._compute_returns(result_df)
        equity = result_df["equity"].to_list()
        trade_pnl = trades_df["pnl"].to_list() if len(trades_df) > 0 else [0.0]

        metrics = compute_all(
            returns=returns,
            equity=equity,
            trades_pnl=trade_pnl,
            initial_capital=self.config.initial_capital,
        )

        pnl_df = result_df.select(["ts", "equity"]).with_columns(
            (pl.col("equity").diff().fill_null(0)).alias("pnl")
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
        compiled: dict[str, object] = {}
        for name, expr in self.config.factors.items():
            node = parse(expr)
            op = compile_expr(node, _FIELDS)
            compiled[name] = op
        return compiled

    # ── Per-bar event loop ──

    def _step_bars(self, factors: dict[str, object]) -> list[dict[str, object]]:
        """Event-driven bar loop with next-bar-open execution."""
        records: list[dict[str, object]] = []
        equity = self.config.initial_capital
        position = 0.0
        entry_price = 0.0
        lots = self._get_lots()
        symbol = self.config.symbols[0] if self.config.symbols else "UNKNOWN"
        bp = self.config.broker_params.get(symbol, BrokerParams())

        n = len(self.bars)
        # Pre-extract bar data for efficiency
        closes = self.bars["close"].to_list()
        opens = self.bars["open"].to_list()
        timestamps = self.bars["ts"].to_list()

        # Pending orders: signal produced at bar i, executed at bar i+1
        pending_entry: dict | None = None
        pending_exit: dict | None = None

        for i in range(n):
            close_px = float(closes[i])
            open_px = float(opens[i])
            ts = timestamps[i]

            # Step 1: Execute pending orders at this bar's open
            trade_pnl = 0.0

            if pending_exit is not None:
                # Exit existing position at this bar's open + slippage
                ex = pending_exit
                exec_px = _apply_slippage(open_px, ex["side"], bp, self.config.fees)
                trade_pnl = compute_trade_pnl(
                    ex["entry_price"], exec_px, ex["lots"], ex["side"], bp,
                )
                equity += trade_pnl
                position = 0.0
                pending_exit = None

            if pending_entry is not None and position == 0:
                # Enter new position at this bar's open + slippage
                en = pending_entry
                exec_px = _apply_slippage(open_px, en["side"], bp, self.config.fees)
                position = en["lots"] if en["side"] == "long" else -en["lots"]
                entry_price = exec_px

            pending_entry = None

            # Step 2: Evaluate factors at this bar's close
            factor_vals: dict[str, float] = {}
            for name, op in factors.items():
                val = op.eval(close_px)
                factor_vals[name] = val

            # Step 3: Compute signal
            signal_val = self._compute_signal(factor_vals)

            # Step 4: Generate orders for NEXT bar
            if signal_val > 0 and position <= 0:
                if position < 0:
                    # Need to exit short first, then enter long
                    pending_exit = {
                        "side": "sell", "entry_price": entry_price,
                        "lots": abs(position),
                    }
                pending_entry = {"side": "long", "lots": lots}
            elif signal_val < 0 and position >= 0:
                if position > 0:
                    pending_exit = {
                        "side": "buy", "entry_price": entry_price,
                        "lots": abs(position),
                    }
                pending_entry = {"side": "short", "lots": lots}
            elif signal_val == 0 and position != 0:
                side = "buy" if position > 0 else "sell"
                pending_exit = {
                    "side": side, "entry_price": entry_price,
                    "lots": abs(position),
                }
                pending_entry = None

            records.append({
                "ts": ts,
                "close": close_px,
                "open": open_px,
                "signal": signal_val,
                "position": position,
                "equity": equity,
                "trade_pnl": trade_pnl,
                **factor_vals,
            })

        return records

    def _compute_signal(self, factor_vals: dict[str, float]) -> float:
        rule = self.config.signal_rule.strip()
        if not rule:
            return 0.0
        try:
            node = parse(rule)
            factor_ops = {}
            for name in self.config.factors:
                from alfq_research.factor.dsl.compile import _Const
                factor_ops[name] = _Const(factor_vals.get(name, math.nan))
            op = compile_expr(node, _FIELDS, factors=factor_ops)
            return op.eval(0.0)
        except Exception:
            if rule in factor_vals:
                return factor_vals[rule]
            return 0.0

    # ── Trade extraction ──

    def _extract_trades(self, records: list[dict]) -> pl.DataFrame:
        trades = []
        prev_position = 0.0
        entry_price = 0.0
        entry_ts = None
        entry_side = ""
        prev_equity = self.config.initial_capital

        for i, rec in enumerate(records):
            pos = float(rec["position"])
            eq = float(rec["equity"])
            px = float(rec["close"])
            ts = rec["ts"]

            if prev_position == 0 and pos != 0:
                entry_price = px
                entry_ts = ts
                entry_side = "long" if pos > 0 else "short"
                prev_equity = eq
            elif prev_position != 0 and pos == 0:
                trade_pnl = eq - prev_equity
                trades.append({
                    "entry_ts": entry_ts,
                    "exit_ts": ts,
                    "symbol": self.config.symbols[0] if self.config.symbols else "",
                    "side": entry_side,
                    "lots": abs(prev_position),
                    "entry_price": entry_price,
                    "exit_price": px,
                    "pnl": trade_pnl,
                    "holding_bars": i - (records.index(rec) if entry_ts else 0),
                })
            elif prev_position != 0 and pos != 0 and (
                (prev_position > 0 and pos < 0) or (prev_position < 0 and pos > 0)
            ):
                # Flip: close previous, open new
                trade_pnl = eq - prev_equity
                trades.append({
                    "entry_ts": entry_ts,
                    "exit_ts": ts,
                    "symbol": self.config.symbols[0] if self.config.symbols else "",
                    "side": entry_side,
                    "lots": abs(prev_position),
                    "entry_price": entry_price,
                    "exit_price": px,
                    "pnl": trade_pnl,
                    "holding_bars": 0,
                })
                entry_price = px
                entry_ts = ts
                entry_side = "long" if pos > 0 else "short"
                prev_equity = eq

            prev_position = pos

        if trades:
            return pl.DataFrame(trades)
        return pl.DataFrame(
            schema={
                "entry_ts": pl.Utf8, "exit_ts": pl.Utf8, "symbol": pl.Utf8,
                "side": pl.Utf8, "lots": pl.Float64, "entry_price": pl.Float64,
                "exit_price": pl.Float64, "pnl": pl.Float64, "holding_bars": pl.Int64,
            }
        )

    def _compute_returns(self, result_df: pl.DataFrame) -> list[float]:
        equity = result_df["equity"].to_list()
        returns = []
        for i in range(1, len(equity)):
            if equity[i - 1] != 0:
                returns.append((equity[i] - equity[i - 1]) / equity[i - 1])
            else:
                returns.append(0.0)
        return returns

    def _get_lots(self) -> float:
        sizing = self.config.sizing
        if sizing.get("type") == "fixed_lots":
            return float(sizing.get("lots", 0.1))
        return 0.1


def _apply_slippage(price: float, side: str, bp: BrokerParams, fee: FeeConfig) -> float:
    """Apply slippage to execution price."""
    slip = fee.slippage_points * bp.point
    half_spread = fee.spread_points / 2.0 * bp.point
    if side == "buy":
        return price + slip + half_spread
    return price - slip - half_spread
