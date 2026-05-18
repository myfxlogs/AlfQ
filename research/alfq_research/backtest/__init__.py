"""ALFQ backtest engine — skeleton.

Features planned:
- Vectorized backtest (Polars)
- Event-driven backtest (tick/bar stepping)
- Slippage / commission / swap simulation per docs/14 §4
- Performance metrics: Sharpe, Sortino, MaxDD
"""

from dataclasses import dataclass, field


@dataclass
class BacktestConfig:
    initial_capital: float = 100_000.0
    commission_per_lot: float = 7.0
    slippage_points_mean: float = 0.0
    slippage_points_std: float = 0.0003
    start_ts_ms: int = 0
    end_ts_ms: int = 0


@dataclass
class Trade:
    symbol: str
    side: str  # "buy" or "sell"
    entry_price: float
    exit_price: float = 0.0
    volume: float = 0.0
    entry_ts_ms: int = 0
    exit_ts_ms: int = 0
    pnl: float = 0.0


@dataclass
class BacktestResult:
    trades: list[Trade] = field(default_factory=list)
    equity_curve: list[float] = field(default_factory=list)
    total_return: float = 0.0
    sharpe_ratio: float = 0.0
    max_drawdown: float = 0.0
    win_rate: float = 0.0
    profit_factor: float = 0.0
