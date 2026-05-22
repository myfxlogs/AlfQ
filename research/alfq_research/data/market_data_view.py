"""ALFQ Research — MarketDataView Python Protocol (RS01).

Mirrors the Go marketdataview.CHView interface so that
research SDK and quantengine share the same data source.
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Protocol


@dataclass
class BarView:
    """Single OHLCV bar from ClickHouse, compatible with proto BarView."""
    tenant_id: str
    symbol: str
    period: str
    ts_unix_ms: int
    open: float
    high: float
    low: float
    close: float
    volume: float


@dataclass
class BarQuery:
    """Query parameters for historical bars."""
    tenant_id: str
    symbol: str
    period: str
    from_ms: int = 0
    to_ms: int = 0
    limit: int = 1000


class MarketDataView(Protocol):
    """Python Protocol matching Go's MarketDataViewService.

    Implementations: CHView (ClickHouse), MemoryView (fixtures for testing).
    """

    def bars(self, query: BarQuery) -> list[BarView]:
        """Return historical bars matching the query."""
        ...

    def latest_bar(self, tenant_id: str, symbol: str, period: str) -> BarView | None:
        """Return the most recent bar for a symbol/period pair."""
        ...


class MemoryView:
    """In-memory fixture implementation for testing and parity checks."""

    def __init__(self):
        self._bars: dict[str, list[BarView]] = {}

    def add(self, *bars: BarView) -> None:
        for b in bars:
            key = f"{b.tenant_id}/{b.symbol}/{b.period}"
            self._bars.setdefault(key, []).append(b)

    def bars(self, query: BarQuery) -> list[BarView]:
        key = f"{query.tenant_id}/{query.symbol}/{query.period}"
        all_bars = self._bars.get(key, [])
        filtered = [
            b for b in all_bars
            if (query.from_ms == 0 or b.ts_unix_ms >= query.from_ms)
            and (query.to_ms == 0 or b.ts_unix_ms < query.to_ms)
        ]
        if query.limit > 0:
            filtered = filtered[-query.limit:]
        return filtered

    def latest_bar(self, tenant_id: str, symbol: str, period: str) -> BarView | None:
        key = f"{tenant_id}/{symbol}/{period}"
        bars = self._bars.get(key, [])
        return bars[-1] if bars else None
