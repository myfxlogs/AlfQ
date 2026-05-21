"""ALFQ Research DataClient — unified data access layer for CH / PG / MinIO.

Configuration is read from environment variables (see config.py).  All query
methods return polars DataFrames.
"""

from __future__ import annotations

import os
from dataclasses import dataclass, field
from datetime import datetime
from typing import TYPE_CHECKING

import polars as pl
from clickhouse_connect import get_client as ch_get_client
from loguru import logger

if TYPE_CHECKING:
    from clickhouse_connect.driver import Client as CHClient


def _env(key: str, default: str = "") -> str:
    return os.environ.get(key, default)


@dataclass
class DataClient:
    """Unified data client for CH/PG/MinIO.

    Environment variables
    ---------------------
    ALFQ_CH_HOST / ALFQ_CH_USER / ALFQ_CH_PASSWORD / ALFQ_CH_DB
    ALFQ_PG_DSN
    ALFQ_MINIO_ENDPOINT / ALFQ_MINIO_AK / ALFQ_MINIO_SK / ALFQ_MINIO_BUCKET
    ALFQ_TENANT_ID
    """

    ch_host: str = field(default_factory=lambda: _env("ALFQ_CH_HOST", "localhost"))
    ch_port: int = field(default_factory=lambda: int(_env("ALFQ_CH_PORT", "8123")))
    ch_user: str = field(default_factory=lambda: _env("ALFQ_CH_USER", "default"))
    ch_password: str = field(default_factory=lambda: _env("ALFQ_CH_PASSWORD", ""))
    ch_db: str = field(default_factory=lambda: _env("ALFQ_CH_DB", "alfq"))
    tenant_id: str = field(default_factory=lambda: _env("ALFQ_TENANT_ID", ""))

    _ch: CHClient | None = field(default=None, init=False, repr=False)

    # ------------------------------------------------------------------
    # Connection helpers
    # ------------------------------------------------------------------

    @property
    def ch(self) -> CHClient:
        if self._ch is None:
            self._ch = ch_get_client(
                host=self.ch_host,
                port=self.ch_port,
                username=self.ch_user,
                password=self.ch_password,
                database=self.ch_db,
            )
            logger.debug("ClickHouse connected {}:{}", self.ch_host, self.ch_port)
        return self._ch

    # ------------------------------------------------------------------
    # bars
    # ------------------------------------------------------------------

    def bars(  # noqa: PLR0913
        self,
        symbols: str | list[str],
        period: str = "1m",
        start: str | datetime | None = None,
        end: str | datetime | None = None,
        *,
        broker: str | None = None,
        canonical: bool = True,
    ) -> pl.DataFrame:
        """Load OHLCV bars from ClickHouse ``md_bars``.

        Parameters
        ----------
        symbols:
            Canonical symbol name(s).  When *canonical* is ``True`` (default)
            the query matches on ``canonical`` column; otherwise on ``symbol_raw``.
        period:
            Bar period: ``"1m"``, ``"5m"``, ``"15m"``, ``"1h"``, ``"1d"``, ...
        start / end:
            ISO-8601 strings or aware/unaware ``datetime`` objects.  Times are
            interpreted as UTC.
        broker:
            Optional broker filter.
        canonical:
            Match *symbols* against the ``canonical`` column (True) or
            ``symbol_raw`` (False).

        Returns
        -------
        polars.DataFrame
            Columns: ``symbol``, ``ts``, ``open``, ``high``, ``low``, ``close``,
            ``volume``.
        """  # noqa: E501
        if isinstance(symbols, str):
            symbols = [symbols]

        # Safe column name — only two valid values
        sym_col = "canonical" if canonical else "symbol_raw"

        where: list[str] = []
        params: dict[str, object] = {}

        if self.tenant_id:
            where.append("tenant_id = {tenant:String}")
            params["tenant"] = self.tenant_id

        if len(symbols) == 1:
            where.append("{sym_col:Identifier} = {sym:String}")
            params["sym_col"] = sym_col
            params["sym"] = symbols[0]
        else:
            where.append("{sym_col:Identifier} IN {syms:Array(String)}")
            params["sym_col"] = sym_col
            params["syms"] = symbols

        where.append("period = {period:String}")
        params["period"] = period

        if broker:
            where.append("broker = {broker:String}")
            params["broker"] = broker

        if start:
            ts = _to_ts_ms(start)
            where.append("open_ts_unix_ms >= {start:UInt64}")
            params["start"] = ts
        if end:
            ts = _to_ts_ms(end)
            where.append("open_ts_unix_ms < {end:UInt64}")
            params["end"] = ts

        query = (
            "SELECT "
            "  {sym_col:Identifier} AS symbol, open_ts_unix_ms AS ts, open, high, low, close, volume "
            "FROM md_bars "
            "WHERE " + " AND ".join(where) + " "
            "ORDER BY symbol, ts"
        )

        result = self.ch.query_df(query, parameters=params)
        df = pl.from_pandas(result)

        # Convert ts (uint64 ms) → datetime[μs, UTC]
        if "ts" in df.columns:
            df = df.with_columns(
                pl.from_epoch("ts", time_unit="ms").cast(pl.Datetime("us", "UTC")).alias("ts")
            )
        return df

    # ------------------------------------------------------------------
    # ticks
    # ------------------------------------------------------------------

    def ticks(
        self,
        symbol: str,
        start: str | datetime | None = None,
        end: str | datetime | None = None,
        *,
        broker: str | None = None,
        canonical: bool = True,
    ) -> pl.DataFrame:
        """Load tick data from ClickHouse ``md_ticks``.

        Returns
        -------
        polars.DataFrame
            Columns: ``symbol``, ``ts_unix_ms``, ``arrived_unix_ms``, ``bid``,
            ``ask``, ``bid_volume``, ``ask_volume``.
        """
        sym_col = "canonical" if canonical else "symbol_raw"

        where: list[str] = []
        params: dict[str, object] = {}

        if self.tenant_id:
            where.append("tenant_id = {tenant:String}")
            params["tenant"] = self.tenant_id

        where.append("{sym_col:Identifier} = {sym:String}")
        params["sym_col"] = sym_col
        params["sym"] = symbol

        if broker:
            where.append("broker = {broker:String}")
            params["broker"] = broker

        if start:
            where.append("ts_unix_ms >= {start:UInt64}")
            params["start"] = _to_ts_ms(start)
        if end:
            where.append("ts_unix_ms < {end:UInt64}")
            params["end"] = _to_ts_ms(end)

        query = (
            "SELECT "
            "  {sym_col:Identifier} AS symbol, ts_unix_ms, arrived_unix_ms, "
            "  bid, ask, bid_volume, ask_volume "
            "FROM md_ticks "
            "WHERE " + " AND ".join(where) + " "
            "ORDER BY ts_unix_ms"
        )

        result = self.ch.query_df(query, parameters=params)
        return pl.from_arrow(result.to_arrow())

    # ------------------------------------------------------------------
    # factor values
    # ------------------------------------------------------------------

    def factor_values(
        self,
        factor_name: str,
        symbol: str,
        start: str | datetime | None = None,
        end: str | datetime | None = None,
    ) -> pl.DataFrame:
        """Load pre-computed factor values from ClickHouse.

        Returns
        -------
        polars.DataFrame
            Columns: ``symbol``, ``ts``, ``value``.
        """
        where: list[str] = []
        params: dict[str, object] = {
            "factor": factor_name,
            "sym": symbol,
        }

        if self.tenant_id:
            where.append("tenant_id = {tenant:String}")
            params["tenant"] = self.tenant_id

        where.append("factor_name = {factor:String}")
        where.append("canonical = {sym:String}")

        if start:
            where.append("open_ts_unix_ms >= {start:UInt64}")
            params["start"] = _to_ts_ms(start)
        if end:
            where.append("open_ts_unix_ms < {end:UInt64}")
            params["end"] = _to_ts_ms(end)

        query = (
            "SELECT canonical AS symbol, ts, value "
            "FROM factor_values "
            "WHERE " + " AND ".join(where) + " "
            "ORDER BY ts"
        )

        result = self.ch.query_df(query, parameters=params)
        return pl.from_arrow(result.to_arrow())


# ------------------------------------------------------------------
# helpers
# ------------------------------------------------------------------

def _to_ts_ms(v: str | datetime) -> int:
    """Convert ISO-8601 string or datetime to Unix milliseconds."""
    import datetime as dt_mod

    if isinstance(v, dt_mod.datetime):
        if v.tzinfo is None:
            v = v.replace(tzinfo=dt_mod.timezone.utc)
        return int(v.timestamp() * 1000)

    # Try parsing ISO-8601
    try:
        ts = dt_mod.datetime.fromisoformat(v)
    except ValueError:
        # Try date-only
        ts = dt_mod.datetime.strptime(v, "%Y-%m-%d")
    if ts.tzinfo is None:
        ts = ts.replace(tzinfo=dt_mod.timezone.utc)
    return int(ts.timestamp() * 1000)
