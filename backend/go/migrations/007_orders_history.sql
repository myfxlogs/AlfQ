-- 007: Historical order local sync tables
-- Add orders_history + account_sync_state for incremental order sync.

BEGIN;

-- -----------------------------------------------------------
-- orders_history: unified closed/pending MT order records
-- -----------------------------------------------------------
CREATE TABLE IF NOT EXISTS orders_history (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    account_id      UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    ticket          BIGINT NOT NULL,
    symbol          TEXT NOT NULL,
    side            TEXT NOT NULL,                     -- 'buy' | 'sell'
    lots            NUMERIC(20, 4) NOT NULL,
    open_price      NUMERIC(20, 5) NOT NULL,
    close_price     NUMERIC(20, 5),
    profit          NUMERIC(20, 4) NOT NULL DEFAULT 0,
    swap            NUMERIC(20, 4) NOT NULL DEFAULT 0,
    commission      NUMERIC(20, 4) NOT NULL DEFAULT 0,
    open_time       TIMESTAMPTZ NOT NULL,
    close_time      TIMESTAMPTZ,                       -- NULL = not yet closed
    state           TEXT NOT NULL DEFAULT 'closed',    -- 'pending' | 'open' | 'closed' | 'cancelled'
    raw_payload     JSONB,                             -- raw MT fields for debug
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (account_id, ticket)
);

CREATE INDEX idx_orders_history_account_close_time
    ON orders_history (account_id, close_time DESC NULLS LAST);
CREATE INDEX idx_orders_history_tenant
    ON orders_history (tenant_id);

-- RLS
ALTER TABLE orders_history ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON orders_history
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- -----------------------------------------------------------
-- account_sync_state: per-account sync bookkeeping
-- -----------------------------------------------------------
CREATE TABLE IF NOT EXISTS account_sync_state (
    account_id        UUID PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    last_full_sync_at TIMESTAMPTZ,                     -- last successful full sync
    last_incr_sync_at TIMESTAMPTZ,                     -- last successful incremental / reconcile
    sync_status       TEXT NOT NULL DEFAULT 'idle',    -- 'idle' | 'syncing' | 'error'
    last_error        TEXT,
    total_synced      INTEGER NOT NULL DEFAULT 0,
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- account_sync_state is not tenant-scoped (single-row per account) but
-- still needs a tenant policy for consistency. Use a FK-derived approach.
ALTER TABLE account_sync_state ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON account_sync_state
    USING (account_id IN (
        SELECT id FROM accounts
        WHERE tenant_id = current_setting('app.tenant_id', true)::uuid
    ));

COMMIT;
