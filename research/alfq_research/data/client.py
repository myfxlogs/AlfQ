"""ALFQ Research Data Client — unified data access layer."""
from dataclasses import dataclass



@dataclass
class DataClient:
    """Unified data client for CH/PG/MinIO."""

    ch_addr: str = "localhost:9000"
    pg_dsn: str = "postgres://alfq:alfq_dev@localhost:5432/alfq"
    minio_endpoint: str = "localhost:9002"

    def load_bars(self, symbol: str, period: str, start: str, end: str) -> "list[dict]":
        """Load OHLCV bars from ClickHouse."""
        return []

    def load_ticks(self, symbol: str, start: str, end: str) -> "list[dict]":
        """Load tick data from ClickHouse."""
        return []

    def load_factors(self, factor_name: str, symbol: str) -> "list[dict]":
        """Load factor values from ClickHouse."""
        return []
