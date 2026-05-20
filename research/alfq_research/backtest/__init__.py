"""ALFQ backtest engine.

Features:
- Vectorized backtest (Polars)
- Event-driven backtest (tick/bar stepping)
- Slippage / commission / swap simulation per docs/14 §4
- Performance metrics: Sharpe, Sortino, MaxDD, Calmar, WinRate, ProfitFactor
"""

from .vectorized import BacktestConfig, BacktestResult, VectorizedBacktest
from .runner import BacktestRunner
from .broker_sim import BrokerParams, FeeConfig

__all__ = [
    "BacktestConfig",
    "BacktestResult",
    "VectorizedBacktest",
    "BacktestRunner",
    "BrokerParams",
    "FeeConfig",
]
