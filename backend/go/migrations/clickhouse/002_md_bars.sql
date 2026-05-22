-- 002_md_bars.up.sql
-- OHLCV bar storage, partitioned by month.

CREATE TABLE IF NOT EXISTS alfq.md_bars (
    tenant_id        LowCardinality(String),
    broker           LowCardinality(String),
    symbol_raw       LowCardinality(String),
    canonical        LowCardinality(String),
    period           LowCardinality(String),
    open_ts_unix_ms  UInt64,
    close_ts_unix_ms UInt64,
    open             Decimal(18, 6),
    high             Decimal(18, 6),
    low              Decimal(18, 6),
    close            Decimal(18, 6),
    volume           Float64,
    tick_count       UInt32
) ENGINE = ReplacingMergeTree()
PARTITION BY toYYYYMM(toDate(close_ts_unix_ms / 1000))
ORDER BY (broker, canonical, period, close_ts_unix_ms);
