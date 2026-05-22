"""ALFQ Research — CHView client for MarketDataView protocol (RS01).

Mirrors backend/go/internal/common/marketdataview.CHView.
Uses ClickHouse HTTP interface (port 8123) for bar/ticks queries.
"""
from __future__ import annotations

import os
from dataclasses import dataclass

from alfq_research.data.market_data_view import MarketDataView, BarView, BarQuery  # noqa: F401


@dataclass
class CHViewConfig:
    """ClickHouse connection config."""
    host: str = "localhost"
    http_port: int = 8123
    user: str = "default"
    password: str = ""
    database: str = "alfq"


class CHView:
    """ClickHouse-backed MarketDataView for research.

    Uses CH HTTP endpoint (port 8123) to run SQL queries.
    Compatible with the same md_bars table schema used by Go CHView.
    """

    def __init__(self, cfg: CHViewConfig | None = None):
        self.cfg = cfg or CHViewConfig()
        self._base_url = f"http://{self.cfg.host}:{self.cfg.http_port}"

    def bars(self, query: BarQuery) -> list[BarView]:
        """Return historical OHLCV bars from ClickHouse."""
        import urllib.request
        import json

        sql = f"""
        SELECT tenant_id, symbol, period, ts_unix_ms, open, high, low, close, volume
        FROM md_bars
        WHERE tenant_id = '{query.tenant_id}'
          AND symbol = '{query.symbol}'
          AND period = '{query.period}'
        """
        if query.from_ms > 0:
            sql += f" AND ts_unix_ms >= {query.from_ms}"
        if query.to_ms > 0:
            sql += f" AND ts_unix_ms < {query.to_ms}"
        sql += f" ORDER BY ts_unix_ms ASC"
        if query.limit > 0:
            sql += f" LIMIT {query.limit}"

        params = {
            "query": sql + " FORMAT JSONCompact",
            "user": self.cfg.user,
            "password": self.cfg.password,
            "database": self.cfg.database,
        }
        url = self._base_url + "/?" + urllib.parse.urlencode(params)

        try:
            with urllib.request.urlopen(url, timeout=10) as resp:
                raw = json.loads(resp.read())
        except Exception:
            return []

        bars = []
        if "data" in raw:
            for row in raw["data"]:
                bars.append(BarView(
                    tenant_id=str(row[0]),
                    symbol=str(row[1]),
                    period=str(row[2]),
                    ts_unix_ms=int(row[3]),
                    open=float(row[4]),
                    high=float(row[5]),
                    low=float(row[6]),
                    close=float(row[7]),
                    volume=float(row[8]),
                ))
        return bars

    def latest_bar(self, tenant_id: str, symbol: str, period: str) -> BarView | None:
        """Return the most recent bar for a symbol/period pair."""
        bars = self.bars(BarQuery(
            tenant_id=tenant_id,
            symbol=symbol,
            period=period,
            limit=1,
        ))
        return bars[0] if bars else None

    def ticks(self, tenant_id: str, symbol: str, from_ms: int = 0, limit: int = 1000) -> list[dict]:
        """Return recent ticks from md_ticks table."""
        import urllib.request
        import json

        sql = f"""
        SELECT tenant_id, symbol, ts_unix_ms, bid, ask, volume
        FROM md_ticks
        WHERE tenant_id = '{tenant_id}'
          AND symbol = '{symbol}'
        """
        if from_ms > 0:
            sql += f" AND ts_unix_ms >= {from_ms}"
        sql += f" ORDER BY ts_unix_ms ASC LIMIT {limit}"

        params = {
            "query": sql + " FORMAT JSONCompact",
            "user": self.cfg.user,
            "password": self.cfg.password,
            "database": self.cfg.database,
        }
        url = self._base_url + "/?" + urllib.parse.urlencode(params)

        try:
            with urllib.request.urlopen(url, timeout=10) as resp:
                raw = json.loads(resp.read())
        except Exception:
            return []

        ticks = []
        if "data" in raw:
            for row in raw["data"]:
                ticks.append({
                    "tenant_id": str(row[0]),
                    "symbol": str(row[1]),
                    "ts_unix_ms": int(row[2]),
                    "bid": float(row[3]),
                    "ask": float(row[4]),
                    "volume": float(row[5]),
                })
        return ticks
