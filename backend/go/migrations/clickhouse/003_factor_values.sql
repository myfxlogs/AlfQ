-- factor_values: stores computed factor results per docs/02 §2.4.
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

-- signals: stores strategy signals per docs/02 §2.5.
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
