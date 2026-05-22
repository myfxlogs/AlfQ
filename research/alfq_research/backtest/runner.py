"""ALFQ Backtest Runner — high-level API for running backtests.

Thin wrapper that wires DataClient + VectorizedBacktest + broker params.
"""

from __future__ import annotations

from dataclasses import dataclass

import polars as pl

from .vectorized import BacktestConfig, BacktestResult, VectorizedBacktest
from .broker_sim import BrokerParams


@dataclass
class BacktestRunner:
    """High-level backtest runner.

    Usage::

        runner = BacktestRunner()
        result = runner.run(config, bars)
    """

    def run(self, config: BacktestConfig, bars: pl.DataFrame) -> BacktestResult:
        """Run a backtest with the given config and bar data."""
        engine = VectorizedBacktest(config, bars)
        return engine.run()

    def run_from_data_client(
        self,
        config: BacktestConfig,
        broker_id: str = "",
    ) -> BacktestResult:
        """Load bars from DataClient and run backtest.

        Fetches broker params from PG if broker_id is set and broker_params
        are not already provided.
        """
        from alfq_research.data.client import DataClient

        dc = DataClient()

        # Load bars
        bars = dc.bars(
            symbols=config.symbols,
            period=config.period,
            start=config.start,
            end=config.end,
            broker=broker_id or None,
        )

        # Fetch broker params if needed
        if broker_id and not config.broker_params:
            self._load_broker_params(config, broker_id)

        return self.run(config, bars)

    @staticmethod
    def _load_broker_params(config: BacktestConfig, broker_id: str) -> None:
        """Load broker params from PG and attach to config."""
        import os
        import psycopg

        dsn = os.environ.get("ALFQ_PG_DSN", "postgresql://alfq:alfq_dev@localhost:5432/alfq")

        try:
            with psycopg.connect(dsn, autocommit=True) as conn, conn.cursor() as cur:
                cur.execute(
                    """SELECT canonical, contract_size, min_lot, max_lot, lot_step,
                              tick_size, tick_value, point, swap_long, swap_short,
                              COALESCE(swap_mode,0), digits
                       FROM broker_symbols
                       WHERE broker_id = %s AND canonical = ANY(%s)""",
                    (broker_id, config.symbols),
                )
                for row in cur.fetchall():
                    canonical = row[0]
                    config.broker_params[canonical] = BrokerParams(
                        contract_size=float(row[1]),
                        min_lot=float(row[2]),
                        max_lot=float(row[3]),
                        lot_step=float(row[4]),
                        tick_size=float(row[5]),
                        tick_value=float(row[6]),
                        point=float(row[7]),
                        swap_long=float(row[8]),
                        swap_short=float(row[9]),
                        swap_mode=int(row[10]),
                        digits=int(row[11]),
                    )
        except ImportError:
            pass  # psycopg not available, use defaults
