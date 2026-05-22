-- 017: Strategy runtime snapshots for crash recovery (RS05).
CREATE TABLE IF NOT EXISTS strategy_runtime_snapshots (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    strategy_name   text NOT NULL,
    revision_id     text NOT NULL DEFAULT '',
    state_json      jsonb NOT NULL,
    snapshot_at_ms  bigint NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_runtime_snapshots_strategy
    ON strategy_runtime_snapshots (strategy_name, snapshot_at_ms DESC);
