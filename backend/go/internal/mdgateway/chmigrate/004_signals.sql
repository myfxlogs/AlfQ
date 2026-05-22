-- 004_signals.sql
-- Strategy signal storage per docs/02 §2.5, partitioned by month.

CREATE TABLE IF NOT EXISTS alfq.signals (
    tenant_id     LowCardinality(String) NOT NULL,
    strategy_id   String,
    deployment_id String,
    symbol        LowCardinality(String),
    ts            DateTime64(3, 'UTC'),
    side          Int8,
    target_qty    Float64,
    limit_price   Nullable(Float64),
    client_id     String,
    created_at    DateTime64(3, 'UTC') DEFAULT now64(3)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(ts)
ORDER BY (tenant_id, strategy_id, ts)
TTL ts + INTERVAL 2 YEAR;
