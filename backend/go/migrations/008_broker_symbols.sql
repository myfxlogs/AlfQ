-- 008_broker_symbols.sql
-- Broker symbol metadata, per (broker, symbol_raw).
-- Populated by symbolsync on account connect + periodic 6h refresh.
-- Refs: docs/tasks/MASTER-ROADMAP.md §2.2.2, SM-1.

CREATE TABLE IF NOT EXISTS broker_symbols (
    broker_id           UUID NOT NULL,
    symbol_raw          TEXT NOT NULL,           -- broker原始名 (EURUSD.m)
    canonical           TEXT NOT NULL,           -- 规范名  (EURUSD)
    digits              SMALLINT NOT NULL,
    point               DOUBLE PRECISION,
    tick_size           DOUBLE PRECISION,
    tick_value          DOUBLE PRECISION,
    contract_size       DOUBLE PRECISION,
    min_lot             DOUBLE PRECISION,
    max_lot             DOUBLE PRECISION,
    lot_step            DOUBLE PRECISION,
    margin_initial      DOUBLE PRECISION,
    margin_currency     TEXT,
    profit_currency     TEXT,
    swap_long           DOUBLE PRECISION,
    swap_short          DOUBLE PRECISION,
    swap_mode           SMALLINT,
    swap_rollover_day   SMALLINT,                -- 三倍仓息日 (1=Mon..5=Fri)
    trade_mode          SMALLINT,                -- 0 disabled / 1 long_only / 2 short_only / 3 full
    description         TEXT,
    sessions_quote      JSONB,                   -- 报价时段 7 天
    sessions_trade      JSONB,                   -- 交易时段 7 天
    server_timezone     TEXT,
    raw_payload         JSONB,                   -- 原始 MT 响应，做溯源
    partial             BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (broker_id, symbol_raw)
);
CREATE INDEX IF NOT EXISTS idx_broker_symbols_canonical ON broker_symbols(canonical);
CREATE INDEX IF NOT EXISTS idx_broker_symbols_updated   ON broker_symbols(updated_at);

-- canonical 映射覆盖 (人工锁定特殊命名)
CREATE TABLE IF NOT EXISTS symbol_canonical_overrides (
    broker_id  UUID NOT NULL,
    symbol_raw TEXT NOT NULL,
    canonical  TEXT NOT NULL,
    note       TEXT,
    PRIMARY KEY (broker_id, symbol_raw)
);
