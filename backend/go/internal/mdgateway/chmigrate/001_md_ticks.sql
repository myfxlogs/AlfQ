-- 001_md_ticks.up.sql
-- md-gateway tick storage, partitioned by month, 90-day TTL.

CREATE TABLE IF NOT EXISTS alfq.md_ticks (
    tenant_id        LowCardinality(String),
    broker           LowCardinality(String),
    symbol_raw       LowCardinality(String),
    canonical        LowCardinality(String),
    ts_unix_ms       UInt64,
    arrived_unix_ms  UInt64,
    bid              Decimal(18, 6),
    ask              Decimal(18, 6),
    bid_volume       Float64,
    ask_volume       Float64
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(toDate(ts_unix_ms / 1000))
ORDER BY (broker, canonical, ts_unix_ms)
TTL toDate(ts_unix_ms / 1000) + INTERVAL 90 DAY;
