-- ALFQ M0-M6 Schema Migrations
-- Run in order. UP only (goose).

-- 001: Core tables
CREATE TABLE IF NOT EXISTS users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  email TEXT NOT NULL,
  password_hash TEXT NOT NULL,
  roles TEXT[] DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS feature_flags (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  key TEXT NOT NULL UNIQUE,
  description TEXT,
  type TEXT NOT NULL DEFAULT 'bool',
  default_val JSONB NOT NULL DEFAULT 'false',
  rollout JSONB NOT NULL DEFAULT '{}',
  owner TEXT,
  expires_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 002: Broker & Account
CREATE TABLE IF NOT EXISTS brokers (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  code TEXT NOT NULL,
  name TEXT NOT NULL,
  platform TEXT NOT NULL,
  mtapi_endpoint TEXT NOT NULL,
  default_server TEXT,
  UNIQUE(tenant_id, code)
);

CREATE TABLE IF NOT EXISTS accounts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  broker_id UUID NOT NULL REFERENCES brokers(id),
  login TEXT NOT NULL,
  password TEXT NOT NULL,
  server TEXT NOT NULL,
  account_type TEXT NOT NULL DEFAULT 'demo',
  position_mode TEXT NOT NULL DEFAULT 'auto',
  currency TEXT NOT NULL DEFAULT 'USD',
  leverage INT DEFAULT 100,
  UNIQUE(broker_id, login)
);

-- 003: Strategy & Deployment
CREATE TABLE IF NOT EXISTS strategies (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  name TEXT NOT NULL,
  description TEXT,
  spec JSONB NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 004: Orders & Positions
CREATE TABLE IF NOT EXISTS orders (
  order_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  account_id UUID NOT NULL,
  strategy_id UUID,
  client_order_id TEXT NOT NULL,
  broker_ticket TEXT,
  symbol TEXT NOT NULL,
  side INT NOT NULL,
  type INT NOT NULL,
  state INT NOT NULL,
  price NUMERIC(20,8),
  stop_price NUMERIC(20,8),
  qty NUMERIC(12,4) NOT NULL,
  filled_qty NUMERIC(12,4) DEFAULT 0,
  avg_fill_price NUMERIC(20,8),
  created_ts_ms BIGINT NOT NULL,
  updated_ts_ms BIGINT NOT NULL,
  UNIQUE(account_id, client_order_id)
);

CREATE TABLE IF NOT EXISTS positions (
  position_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  account_id UUID NOT NULL,
  symbol TEXT NOT NULL,
  qty NUMERIC(12,4) NOT NULL,
  avg_price NUMERIC(20,8),
  UNIQUE(account_id, symbol)
);

-- 005: Risk events
CREATE TABLE IF NOT EXISTS risk_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  event_type TEXT NOT NULL,
  account_id UUID,
  symbol TEXT,
  rule_id TEXT,
  reason TEXT,
  by_user TEXT,
  ts_unix_ms BIGINT NOT NULL
);

-- 006: ACL
CREATE TABLE IF NOT EXISTS acl (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  resource_type TEXT NOT NULL,
  resource_id UUID NOT NULL,
  action TEXT NOT NULL,
  UNIQUE(user_id, resource_type, resource_id, action)
);

-- 007: RLS policies
ALTER TABLE orders ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON orders
  USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

ALTER TABLE accounts ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON accounts
  USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- Seed: demo admin user (a@1.com / 12345678)
INSERT INTO users (id, tenant_id, email, password_hash, roles) VALUES (
  '00000000-0000-0000-0000-000000000001',
  '00000000-0000-0000-0000-000000000001',
  'a@1.com',
  '$argon2id$v=19$m=65536,t=3,p=4$Ssl7GTFwuuWAMk07VJFseA$BEbTU0MRUbRTnLesUU3jJs6XdCGbRODlG7droYgJ6vM',
  ARRAY['admin']
) ON CONFLICT (id) DO NOTHING;
