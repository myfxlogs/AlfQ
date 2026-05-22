-- 010_risk_events_enhance.sql
-- Add severity, strategy_id, and order_request_json columns to risk_events.
-- Create risk_limits table for per-tenant per-account risk thresholds.

ALTER TABLE risk_events
    ADD COLUMN IF NOT EXISTS severity text NOT NULL DEFAULT 'P2',
    ADD COLUMN IF NOT EXISTS strategy_id uuid,
    ADD COLUMN IF NOT EXISTS order_request_json jsonb;

-- risk_limits: per-account risk thresholds, overrides system defaults.
CREATE TABLE IF NOT EXISTS risk_limits (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  uuid NOT NULL,
    account_id uuid,                          -- NULL = tenant-wide default
    strategy_id uuid,                         -- NULL = all strategies
    rule_id    text NOT NULL,                 -- "max_lot", "daily_loss", "drawdown", ...
    threshold  jsonb NOT NULL,                -- {"max_lot": 100, "daily_loss": 5000, ...}
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Default risk limits for the default tenant
INSERT INTO risk_limits (tenant_id, rule_id, threshold)
VALUES
    ('00000000-0000-0000-0000-000000000001', 'max_lot',       '{"max_lot": 100}'),
    ('00000000-0000-0000-0000-000000000001', 'daily_loss',    '{"max_daily_loss": 5000}'),
    ('00000000-0000-0000-0000-000000000001', 'drawdown',      '{"max_drawdown": 0.15}'),
    ('00000000-0000-0000-0000-000000000001', 'max_position',  '{"max_per_symbol": 10}'),
    ('00000000-0000-0000-0000-000000000001', 'margin',        '{"min_margin_level": 1.5}')
ON CONFLICT DO NOTHING;

-- Index for promotion gate check (RC04 acceptance)
CREATE INDEX IF NOT EXISTS idx_risk_events_strategy_severity
    ON risk_events (strategy_id, severity, ts_unix_ms DESC);
