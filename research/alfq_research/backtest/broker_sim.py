"""ALFQ Backtest — broker simulation (slippage, commission, swap).

Models per docs/14-领域模型与交易规则.md §3-4.
RS04: SpreadModel, FillModel, three-tier fee config, broker_symbols loader.
"""

from __future__ import annotations

import random
from dataclasses import dataclass, field
from enum import Enum
from typing import Protocol


# ═══════════════════════════════════════════════════════════════════════
# Enums
# ═══════════════════════════════════════════════════════════════════════

class FeeTier(Enum):
    """Three-tier fee model per RS04."""
    OPTIMISTIC = "optimistic"
    REALISTIC = "realistic"
    CONSERVATIVE = "conservative"


class Session(Enum):
    """Trading session for time-varying spread."""
    ASIAN = "asian"        # 00:00-08:00 UTC
    EUROPEAN = "european"  # 08:00-16:00 UTC
    AMERICAN = "american"  # 13:00-21:00 UTC
    NEWS = "news"          # high-impact news events
    WEEKEND = "weekend"    # market closed


# ═══════════════════════════════════════════════════════════════════════
# Protocols
# ═══════════════════════════════════════════════════════════════════════

class BrokerSymbolsLoader(Protocol):
    """Protocol for loading per-symbol broker parameters from broker_symbols table."""
    def load(self, symbol: str) -> BrokerParams | None: ...


# ═══════════════════════════════════════════════════════════════════════
# BrokerParams
# ═══════════════════════════════════════════════════════════════════════

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
    swap_mode: int = 0       # 0=Points, 2=Currency (RS04)
    digits: int = 5
    commission_per_lot: float = 7.0
    spread_avg: float = 1.0   # average spread in points
    slippage_avg: float = 0.5  # average slippage in points


# ═══════════════════════════════════════════════════════════════════════
# FeeConfig
# ═══════════════════════════════════════════════════════════════════════

@dataclass
class FeeConfig:
    """Trading cost configuration. Load from broker_symbols in production."""
    commission_per_lot: float = 7.0
    slippage_points: float = 0.5
    spread_points: float = 1.0
    enable_swap: bool = True
    tier: FeeTier = FeeTier.REALISTIC

    @classmethod
    def from_symbol(cls, bp: BrokerParams, spread_avg: float | None = None, slippage_avg: float | None = None, tier: FeeTier = FeeTier.REALISTIC) -> "FeeConfig":
        """Create FeeConfig from real broker_symbols data (RS04)."""
        cfg = cls(tier=tier, commission_per_lot=bp.commission_per_lot)
        if spread_avg is not None and spread_avg > 0:
            cfg.spread_points = spread_avg / bp.point if bp.point > 0 else spread_avg
        elif bp.spread_avg > 0:
            cfg.spread_points = bp.spread_avg / bp.point if bp.point > 0 else bp.spread_avg
        if slippage_avg is not None and slippage_avg > 0:
            cfg.slippage_points = slippage_avg / bp.point if bp.point > 0 else slippage_avg
        elif bp.slippage_avg > 0:
            cfg.slippage_points = bp.slippage_avg / bp.point if bp.point > 0 else bp.slippage_avg
        # Apply tier multiplier
        cfg._apply_tier()
        return cfg

    @classmethod
    def optimistic(cls) -> "FeeConfig":
        return cls(commission_per_lot=3.5, slippage_points=0.2, spread_points=0.5, tier=FeeTier.OPTIMISTIC)

    @classmethod
    def realistic(cls) -> "FeeConfig":
        return cls(commission_per_lot=7.0, slippage_points=0.5, spread_points=1.0, tier=FeeTier.REALISTIC)

    @classmethod
    def conservative(cls) -> "FeeConfig":
        return cls(commission_per_lot=14.0, slippage_points=2.0, spread_points=3.0, tier=FeeTier.CONSERVATIVE)

    def _apply_tier(self) -> None:
        """Apply tier-based multipliers to raw broker_symbols values."""
        multipliers = {
            FeeTier.OPTIMISTIC:   {"commission": 0.5, "slippage": 0.4, "spread": 0.5},
            FeeTier.REALISTIC:    {"commission": 1.0, "slippage": 1.0, "spread": 1.0},
            FeeTier.CONSERVATIVE: {"commission": 2.0, "slippage": 4.0, "spread": 3.0},
        }
        m = multipliers.get(self.tier, multipliers[FeeTier.REALISTIC])
        self.commission_per_lot *= m["commission"]
        self.slippage_points *= m["slippage"]
        self.spread_points *= m["spread"]

    @property
    def half_spread_cost(self) -> float:
        return self.spread_points / 2.0


# ═══════════════════════════════════════════════════════════════════════
# SpreadModel
# ═══════════════════════════════════════════════════════════════════════

@dataclass
class SpreadDistribution:
    """Per-session spread statistics (in points)."""
    mean: float
    std: float
    p50: float
    p95: float
    p99: float

    def sample(self) -> float:
        """Sample a spread value from a log-normal approximation."""
        if self.std <= 0:
            return self.mean
        # Use log-normal to keep values positive
        mu = max(0.01, self.mean)
        sigma = self.std / mu if mu > 0 else 0.3
        return random.lognormvariate(mu, sigma)


class SpreadModel:
    """Time-varying spread model using per-session distributions.

    Loaded from ClickHouse historical tick data. Falls back to
    default spreads when no data is available.
    """

    def __init__(self, symbol: str, fee: FeeConfig | None = None):
        self.symbol = symbol
        self.fee = fee or FeeConfig()
        self._distributions: dict[Session, SpreadDistribution] = {}

    def load(self, session: Session, dist: SpreadDistribution) -> None:
        """Register a spread distribution for a session."""
        self._distributions[session] = dist

    def load_defaults(self) -> None:
        """Populate reasonable defaults for major forex pairs."""
        base = self.fee.spread_points
        self._distributions = {
            Session.ASIAN:    SpreadDistribution(mean=base * 1.3, std=base * 0.2, p50=base * 1.3, p95=base * 1.8, p99=base * 2.2),
            Session.EUROPEAN: SpreadDistribution(mean=base * 0.8, std=base * 0.15, p50=base * 0.8, p95=base * 1.1, p99=base * 1.4),
            Session.AMERICAN: SpreadDistribution(mean=base * 0.9, std=base * 0.2, p50=base * 0.9, p95=base * 1.3, p99=base * 1.8),
            Session.NEWS:     SpreadDistribution(mean=base * 3.0, std=base * 1.5, p50=base * 2.5, p95=base * 5.0, p99=base * 8.0),
            Session.WEEKEND:  SpreadDistribution(mean=base * 0.5, std=base * 0.05, p50=base * 0.5, p95=base * 0.6, p99=base * 0.7),
        }

    def get_spread(self, session: Session | None = None, timestamp_unix_ms: int | None = None) -> float:
        """Return spread in points for the given session/timestamp."""
        s = session or _session_from_timestamp(timestamp_unix_ms or 0)
        dist = self._distributions.get(s)
        if dist is None:
            return self.fee.spread_points
        return max(0.1, dist.sample())

    @classmethod
    def from_fee(cls, symbol: str, fee: FeeConfig) -> "SpreadModel":
        sm = cls(symbol, fee)
        sm.load_defaults()
        return sm


def _session_from_timestamp(ts_ms: int) -> Session:
    """Map a UTC millisecond timestamp to a trading session."""
    import datetime
    dt = datetime.datetime.fromtimestamp(ts_ms / 1000, tz=datetime.timezone.utc)
    hour = dt.hour
    weekday = dt.weekday()  # 0=Mon, 6=Sun
    if weekday >= 5:
        return Session.WEEKEND
    if 0 <= hour < 8:
        return Session.ASIAN
    if 8 <= hour < 13:
        return Session.EUROPEAN
    # 13:00-21:00 UTC overlaps European afternoon + American morning
    if 13 <= hour < 21:
        return Session.AMERICAN
    return Session.EUROPEAN


# ═══════════════════════════════════════════════════════════════════════
# FillModel
# ═══════════════════════════════════════════════════════════════════════

@dataclass
class FillResult:
    """Result of a fill simulation."""
    filled_lots: float
    fill_price: float
    slippage_cost: float
    spread_cost: float
    commission: float
    rejected: bool = False
    reject_reason: str = ""


class FillModel:
    """Simulates order execution with partial fills, SL/TP, and limit orders.

    Per docs/14 §PR-3: limit orders fill when price reaches the limit level.
    SL/TP triggers when bar high/low crosses the level; if both trigger in
    the same bar, the conservative (worst-case first) outcome is chosen.
    """

    def __init__(self, bp: BrokerParams, fee: FeeConfig, spread_model: SpreadModel | None = None):
        self.bp = bp
        self.fee = fee
        self.spread_model = spread_model or SpreadModel.from_fee("", fee)
        self._rng = random.Random()

    def market_order(
        self,
        side: str,
        lots: float,
        bar_open: float,
        bar_high: float,
        bar_low: float,
        bar_close: float,
        volume: float = 0,
        timestamp_ms: int = 0,
    ) -> FillResult:
        """Simulate a market order fill.

        Uses bar close as reference price, applies spread and slippage.
        Partial fills based on volume proxy (lower volume → higher chance of partial fill).
        """
        session = _session_from_timestamp(timestamp_ms)
        spread_pts = self.spread_model.get_spread(session, timestamp_ms)
        slippage_pts = self.fee.slippage_points

        # Reference price: weighted toward close with some randomness
        ref_price = bar_close * (1 + self._rng.uniform(-0.0001, 0.0001))

        # Spread cost
        spread_cost = spread_pts * self.bp.point
        slip_cost = slippage_pts * self.bp.point

        if side == "buy":
            fill_price = ref_price + spread_cost / 2 + slip_cost
        else:
            fill_price = ref_price - spread_cost / 2 - slip_cost

        # Partial fill probability based on volume (if available)
        filled = lots
        if volume > 0:
            # Simple liquidity model: if lots > some threshold, partial fill
            liquidity_threshold = max(0.1, volume * 0.01)
            if lots > liquidity_threshold:
                fill_ratio = self._rng.uniform(0.3, 1.0)
                filled = max(self.bp.min_lot, round(lots * fill_ratio / self.bp.lot_step) * self.bp.lot_step)

        commission = abs(filled) * self.fee.commission_per_lot

        return FillResult(
            filled_lots=filled,
            fill_price=fill_price,
            slippage_cost=slip_cost * abs(filled) * self.bp.contract_size,
            spread_cost=spread_cost * abs(filled) * self.bp.contract_size,
            commission=commission,
        )

    def limit_order(
        self,
        side: str,
        lots: float,
        limit_price: float,
        bar_open: float,
        bar_high: float,
        bar_low: float,
        bar_close: float,
        timestamp_ms: int = 0,
    ) -> FillResult:
        """Simulate a limit order fill per docs/14 §PR-3.

        Limit buy fills when bar_low <= limit_price.
        Limit sell fills when bar_high >= limit_price.
        """
        triggered = False
        if side == "buy" and bar_low <= limit_price:
            triggered = True
        elif side == "sell" and bar_high >= limit_price:
            triggered = True

        if not triggered:
            return FillResult(filled_lots=0, fill_price=0, slippage_cost=0, spread_cost=0, commission=0)

        session = _session_from_timestamp(timestamp_ms)
        spread_pts = self.spread_model.get_spread(session, timestamp_ms)

        # Fill at limit price (best case), minus half spread
        if side == "buy":
            fill_price = limit_price + spread_pts * self.bp.point / 2
        else:
            fill_price = limit_price - spread_pts * self.bp.point / 2

        commission = abs(lots) * self.fee.commission_per_lot
        return FillResult(
            filled_lots=lots,
            fill_price=fill_price,
            slippage_cost=0,  # limit orders have no slippage
            spread_cost=spread_pts * self.bp.point * abs(lots) * self.bp.contract_size / 2,
            commission=commission,
        )

    def check_sl_tp(
        self,
        side: str,
        lots: float,
        entry_price: float,
        sl_price: float | None,
        tp_price: float | None,
        bar_open: float,
        bar_high: float,
        bar_low: float,
        bar_close: float,
    ) -> FillResult | None:
        """Check if SL or TP triggered in this bar.

        Per docs/14: if both SL and TP trigger in the same bar,
        the conservative outcome (worst-case first) is chosen.
        """
        sl_hit = False
        tp_hit = False

        if sl_price is not None and sl_price > 0:
            if side == "buy":
                sl_hit = bar_low <= sl_price
            else:
                sl_hit = bar_high >= sl_price

        if tp_price is not None and tp_price > 0:
            if side == "buy":
                tp_hit = bar_high >= tp_price
            else:
                tp_hit = bar_low <= tp_price

        if not sl_hit and not tp_hit:
            return None

        # Conservative: if both hit, assume worst-case first (SL triggers before TP)
        if sl_hit and tp_hit:
            # Pick the outcome that gives worse PnL
            # SL is always worse (loss) compared to TP (profit)
            tp_hit = False

        if sl_hit:
            fill_price = sl_price if sl_price else bar_close
            return FillResult(
                filled_lots=lots,
                fill_price=fill_price,
                slippage_cost=self.fee.slippage_points * self.bp.point * abs(lots) * self.bp.contract_size,
                spread_cost=0,
                commission=abs(lots) * self.fee.commission_per_lot,
            )

        if tp_hit:
            fill_price = tp_price if tp_price else bar_close
            return FillResult(
                filled_lots=lots,
                fill_price=fill_price,
                slippage_cost=0,
                spread_cost=0,
                commission=abs(lots) * self.fee.commission_per_lot,
            )

        return None


# ═══════════════════════════════════════════════════════════════════════
# Cost computation functions
# ═══════════════════════════════════════════════════════════════════════

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
    """Swap (overnight interest). RS04: corrected per swap_mode.

    - mode=0 (Points): swap_rate * point_value * lots * holding_days
    - mode=2 (Currency): swap_rate * lots * holding_days (already in account currency)
    """
    if not fee.enable_swap or holding_bars <= 0:
        return 0.0
    swap_rate = bp.swap_long if side == "buy" else bp.swap_short
    if bp.swap_mode == 2:
        # Mode 2 (Currency): swap_rate is already in account currency per lot
        return abs(lots) * swap_rate * holding_bars
    # Mode 0 (Points): multiply by point value to get account currency
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
