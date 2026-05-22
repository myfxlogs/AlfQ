"""ALFQ backtest engine.

Features:
- Vectorized backtest (Polars)
- Event-driven backtest (tick/bar stepping)
- Slippage / commission / swap simulation per docs/14 §4
- Performance metrics: Sharpe, Sortino, MaxDD, Calmar, WinRate, ProfitFactor
"""

from .vectorized import BacktestConfig, BacktestResult, VectorizedBacktest
from .event import EventBacktest
from .runner import BacktestRunner
from .broker_sim import (
    BrokerParams,
    FeeConfig,
    FeeTier,
    Session,
    BrokerSymbolsLoader,
    SpreadDistribution,
    SpreadModel,
    FillModel,
    FillResult,
    compute_commission,
    compute_slippage,
    compute_swap,
    compute_trade_pnl,
)
from .consistency import consistency_check

__all__ = [
    "BacktestConfig",
    "BacktestResult",
    "VectorizedBacktest",
    "EventBacktest",
    "BacktestRunner",
    "BrokerParams",
    "FeeConfig",
    "FeeTier",
    "Session",
    "BrokerSymbolsLoader",
    "SpreadDistribution",
    "SpreadModel",
    "FillModel",
    "FillResult",
    "compute_commission",
    "compute_slippage",
    "compute_swap",
    "compute_trade_pnl",
    "consistency_check",
]
