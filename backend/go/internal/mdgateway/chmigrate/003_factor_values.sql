-- 003_factor_values.sql
-- Factor value storage per docs/02 §2.4, partitioned by month.

CREATE TABLE IF NOT EXISTS alfq.factor_values (
    tenant_id    LowCardinality(String) NOT NULL,
    account_id   LowCardinality(String),
    symbol       LowCardinality(String),
    factor_name  LowCardinality(String),
    value        Float64,
    ts_ms        Int64,
    created_at   DateTime64(3, 'UTC') DEFAULT now64(3)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(created_at)
ORDER BY (tenant_id, factor_name, symbol, ts_ms)
TTL created_at + INTERVAL 2 YEAR;
