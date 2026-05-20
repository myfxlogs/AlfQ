-- 005_add_account_user_id.sql
-- 添加 user_id 列，实现用户级别的交易账号归属隔离
-- 每个用户只能查看和管理自己绑定的交易账号

BEGIN;

-- 1. 占位 broker：用于直接连接（不通过 broker 查找的直接绑定）
INSERT INTO brokers (id, tenant_id, code, name, platform, mtapi_endpoint)
VALUES ('00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000001', 'direct', 'direct', 'mt5', '')
ON CONFLICT (id) DO NOTHING;

-- 2. 添加 user_id 列（先可空，设默认值，再 NOT NULL）
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS user_id UUID;
UPDATE accounts SET user_id = '00000000-0000-0000-0000-000000000001' WHERE user_id IS NULL;
ALTER TABLE accounts ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE accounts ADD CONSTRAINT accounts_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id);

-- 3. 替换旧的 UNIQUE 约束：同一用户不能重复绑定同一 broker 下同一 login
ALTER TABLE accounts DROP CONSTRAINT IF EXISTS accounts_broker_id_login_key;
ALTER TABLE accounts ADD CONSTRAINT accounts_user_broker_login_key UNIQUE (user_id, broker_id, login);

COMMIT;
