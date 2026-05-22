-- 012: Multi-tenant API key isolation (R10)
-- Each user stores their own LLM provider API key, encrypted at rest.
CREATE TABLE IF NOT EXISTS user_api_keys (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider        text NOT NULL,          -- "openai" | "anthropic"
    model           text NOT NULL DEFAULT '',
    key_cipher      text NOT NULL,          -- AES-256-GCM encrypted
    key_prefix      text NOT NULL,          -- "sk-...****a1b2"
    quota_limit_cents int NOT NULL DEFAULT 500,  -- 月度预算（美分）
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE(user_id, provider)
);
