"""ALFQ Research — PostgreSQL metadata queries."""

from __future__ import annotations

import os
from dataclasses import dataclass

from loguru import logger


def _env(key: str, default: str = "") -> str:
    return os.environ.get(key, default)


@dataclass
class PgClient:
    """Thin wrapper around psycopg for metadata queries.

    Environment: ALFQ_PG_DSN
    """

    dsn: str = _env("ALFQ_PG_DSN", "postgresql://alfq:alfq_dev@localhost:5432/alfq")

    def fetch_broker_params(
        self, broker_id: str, canonical: str
    ) -> dict[str, object] | None:
        """Fetch broker symbol parameters for a (broker, canonical) pair."""
        import psycopg

        try:
            with psycopg.connect(self.dsn, autocommit=True) as conn, conn.cursor() as cur:
                cur.execute(
                    """SELECT contract_size, min_lot, max_lot, lot_step,
                              tick_size, tick_value, point,
                              swap_long, swap_short, digits
                       FROM broker_symbols
                       WHERE broker_id = %s AND canonical = %s
                       LIMIT 1""",
                    (broker_id, canonical),
                )
                row = cur.fetchone()
                if row is None:
                    logger.warning(
                        "broker_symbols not found for broker={} canonical={}",
                        broker_id, canonical,
                    )
                    return None
                cols = [
                    "contract_size", "min_lot", "max_lot", "lot_step",
                    "tick_size", "tick_value", "point",
                    "swap_long", "swap_short", "digits",
                ]
                return dict(zip(cols, row, strict=True))
        except ImportError:
            logger.warning("psycopg not installed; returning default broker params")
            return None
        except Exception as exc:
            logger.warning("PG query failed: {}", exc)
            return None

    def fetch_broker_params_multi(
        self, broker_id: str, canonicals: list[str]
    ) -> dict[str, dict[str, object]]:
        """Batch fetch broker params for multiple symbols."""
        import psycopg

        try:
            with psycopg.connect(self.dsn, autocommit=True) as conn, conn.cursor() as cur:
                cur.execute(
                    """SELECT canonical, contract_size, min_lot, max_lot, lot_step,
                              tick_size, tick_value, point,
                              swap_long, swap_short, digits
                       FROM broker_symbols
                       WHERE broker_id = %s AND canonical = ANY(%s)""",
                    (broker_id, canonicals),
                )
                result: dict[str, dict[str, object]] = {}
                cols = [
                    "contract_size", "min_lot", "max_lot", "lot_step",
                    "tick_size", "tick_value", "point",
                    "swap_long", "swap_short", "digits",
                ]
                for row in cur.fetchall():
                    canonical = row[0]
                    result[canonical] = dict(zip(cols, row[1:], strict=True))
                return result
        except ImportError:
            logger.warning("psycopg not installed; returning empty broker params")
            return {}
        except Exception as exc:
            logger.warning("PG batch query failed: {}", exc)
            return {}
