-- 006_add_account_platform.sql
-- 添加 platform 列，解决重连时依赖 brokers 表导致 MT4/MT5 识别错误
-- 占位 broker 'direct' 固定为 mt5，无法区分实际平台类型

BEGIN;

ALTER TABLE accounts ADD COLUMN IF NOT EXISTS platform TEXT NOT NULL DEFAULT 'mt5';

-- 从 brokers 表回填已有数据（如果有 broker 记录的）
UPDATE accounts a SET platform = b.platform
FROM brokers b
WHERE a.broker_id = b.id AND a.broker_id != '00000000-0000-0000-0000-000000000000';

COMMIT;
