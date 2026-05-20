"""ALFQ Backtest — broker simulation (slippage, commission, swap).

Models per docs/14-领域模型与交易规则.md §3-4.
"""

from __future__ import annotations

from dataclasses import dataclass


@dataclass
class BrokerParams:
    """Per-symbol broker parameters fetched from broker_symbols table."""
    contract_size: float = 100_000.0
    min_lot: float = 0.01
    max_lot: float = 100.0
    lot_step: float = 0.01
    tick_size: float = 0.00001
    tick_value: float = 1.0
    point: float = 0.00001
    swap_long: float = 0.0
    swap_short: float = 0.0
    digits: int = 5


@dataclass
class FeeConfig:
    """Trading cost configuration."""
    commission_per_lot: float = 7.0
    slippage_points: float = 0.5
    spread_points: float = 1.0
    enable_swap: bool = True

    @property
    def half_spread_cost(self) -> float:
        return self.spread_points / 2.0


def compute_commission(lots: float, fee: FeeConfig) -> float:
    return abs(lots) * fee.commission_per_lot


def compute_slippage(price: float, side: str, bp: BrokerParams, fee: FeeConfig) -> float:
    """Effective price adjusted for slippage."""
    slip_abs = fee.slippage_points * bp.point
    spread_half = fee.half_spread_cost * bp.point
    return price + slip_abs + spread_half if side == "buy" else price - slip_abs - spread_half


def compute_swap(
    lots: float, side: str, bp: BrokerParams, fee: FeeConfig, holding_bars: int = 1,
) -> float:
    """Swap (overnight interest)."""
    if not fee.enable_swap or holding_bars <= 0:
        return 0.0
    swap_rate = bp.swap_long if side == "buy" else bp.swap_short
    return abs(lots) * swap_rate * bp.point * holding_bars


def compute_trade_pnl(
    entry_price: float,
    exit_price: float,
    lots: float,
    side: str,
    bp: BrokerParams,
) -> float:
    """Full trade PnL with default FeeConfig. Returns PnL in account currency."""
    fee = FeeConfig()
    price_diff = exit_price - entry_price if side == "buy" else entry_price - exit_price
    gross_pnl = price_diff * lots * bp.contract_size
    commission = compute_commission(lots, fee)
    entry_slip = abs(compute_slippage(entry_price, side, bp, fee) - entry_price)
    exit_side = "sell" if side == "buy" else "buy"
    exit_slip = abs(compute_slippage(exit_price, exit_side, bp, fee) - exit_price)
    slippage_cost = (entry_slip + exit_slip) * abs(lots) * bp.contract_size
    return gross_pnl - commission - slippage_cost
