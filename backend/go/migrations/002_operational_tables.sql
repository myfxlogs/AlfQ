-- ALFQ Migration 002: Post-M0 operational tables
-- Run after 001_initial_schema.sql. UP only.

-- 008: Trading calendar (doc 14)
CREATE TABLE IF NOT EXISTS trading_calendar (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    date DATE NOT NULL,
    is_trading_day BOOLEAN NOT NULL DEFAULT true,
    open_time TIME,
    close_time TIME,
    description TEXT,
    UNIQUE(tenant_id, date)
);

-- 009: Idempotency records (doc 21)
CREATE TABLE IF NOT EXISTS idempotency_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key TEXT NOT NULL UNIQUE,
    response JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);

-- 010: Sagas (doc 21)
CREATE TABLE IF NOT EXISTS sagas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    saga_id TEXT NOT NULL UNIQUE,
    saga_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    steps JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 011: Outbox (doc 21)
CREATE TABLE IF NOT EXISTS outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);

-- 012: AI conversations (doc 18)
CREATE TABLE IF NOT EXISTS ai_conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT,
    model TEXT NOT NULL DEFAULT 'deepseek-v4',
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 013: AI messages (doc 18)
CREATE TABLE IF NOT EXISTS ai_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES ai_conversations(id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    tool_calls JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 014: AI artifacts (doc 18)
CREATE TABLE IF NOT EXISTS ai_artifacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES ai_conversations(id) ON DELETE CASCADE,
    artifact_type TEXT NOT NULL,
    content JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 015: AI quotas (doc 18, doc 23)
CREATE TABLE IF NOT EXISTS ai_quotas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    user_id UUID,
    calls_limit INT NOT NULL DEFAULT 100,
    calls_used INT NOT NULL DEFAULT 0,
    period TEXT NOT NULL DEFAULT 'daily',
    reset_at TIMESTAMPTZ NOT NULL
);

-- 016: Models (doc 18)
CREATE TABLE IF NOT EXISTS models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    name TEXT NOT NULL,
    model_type TEXT NOT NULL,
    version TEXT NOT NULL DEFAULT '1.0.0',
    file_path TEXT NOT NULL,
    metrics JSONB,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 017: Model drift metrics (doc 18)
CREATE TABLE IF NOT EXISTS model_drift_metrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_id UUID NOT NULL REFERENCES models(id) ON DELETE CASCADE,
    drift_score FLOAT8,
    feature_drift JSONB,
    target_drift FLOAT8,
    checked_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 018: User consents (doc 22)
CREATE TABLE IF NOT EXISTS user_consents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    consent_type TEXT NOT NULL,
    granted BOOLEAN NOT NULL DEFAULT false,
    ip_address TEXT,
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, consent_type)
);

-- 019: Data lineage (doc 22)
CREATE TABLE IF NOT EXISTS data_lineage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_table TEXT NOT NULL,
    source_id UUID NOT NULL,
    target_table TEXT,
    target_id UUID,
    transformation TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 020: Plans (doc 23, doc 24)
CREATE TABLE IF NOT EXISTS plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    tier TEXT NOT NULL,
    max_accounts INT NOT NULL DEFAULT 1,
    max_strategies INT NOT NULL DEFAULT 3,
    max_ai_calls_per_day INT NOT NULL DEFAULT 50,
    features JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 021: Tenant quota usage (doc 23)
CREATE TABLE IF NOT EXISTS tenant_quota_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    quota_type TEXT NOT NULL,
    quota_limit INT NOT NULL,
    quota_used INT NOT NULL DEFAULT 0,
    period TEXT NOT NULL DEFAULT 'monthly',
    reset_at TIMESTAMPTZ NOT NULL,
    UNIQUE(tenant_id, quota_type, period)
);

-- 022: Billing events (doc 24)
CREATE TABLE IF NOT EXISTS billing_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    event_type TEXT NOT NULL,
    amount NUMERIC(12,4) NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'USD',
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 023: Support tickets (doc 24)
CREATE TABLE IF NOT EXISTS support_tickets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subject TEXT NOT NULL,
    priority TEXT NOT NULL DEFAULT 'normal',
    status TEXT NOT NULL DEFAULT 'open',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 024: Support messages (doc 24)
CREATE TABLE IF NOT EXISTS support_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id UUID NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
    sender_type TEXT NOT NULL,
    sender_id UUID NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================
-- RLS: Enable row-level security on all tenant-scoped tables
-- ============================================================

-- 025: RLS on users
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON users
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- 026: RLS on feature_flags
ALTER TABLE feature_flags ENABLE ROW LEVEL SECURITY;

-- 027: RLS on brokers
ALTER TABLE brokers ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON brokers
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- 028: RLS on strategies
ALTER TABLE strategies ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON strategies
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- 029: RLS on positions
ALTER TABLE positions ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON positions
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- 030: RLS on risk_events
ALTER TABLE risk_events ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON risk_events
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- 031: RLS on acl
ALTER TABLE acl ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON acl
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- 032: RLS on tenant-scoped new tables
ALTER TABLE trading_calendar ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON trading_calendar
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

ALTER TABLE ai_conversations ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON ai_conversations
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

ALTER TABLE ai_quotas ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON ai_quotas
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

ALTER TABLE models ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON models
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

ALTER TABLE billing_events ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON billing_events
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

ALTER TABLE support_tickets ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON support_tickets
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
