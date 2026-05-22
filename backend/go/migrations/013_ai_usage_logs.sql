-- 013: AI usage tracking with per-user budget enforcement (R10)
CREATE TABLE IF NOT EXISTS ai_usage_logs (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL,
    provider    text NOT NULL,
    model       text NOT NULL,
    tokens_in   int NOT NULL DEFAULT 0,
    tokens_out  int NOT NULL DEFAULT 0,
    cost_cents  int NOT NULL DEFAULT 0,   -- 美分
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_ai_usage_user_date ON ai_usage_logs(user_id, created_at);
