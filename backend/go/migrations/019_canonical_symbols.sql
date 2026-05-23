-- 019_canonical_symbols.sql
-- Multi-broker symbol canonical addressing (M1).
-- Refs: docs/design/multi-broker-symbol.md §3.1
-- Applied: 2026-05-22, 560 canonical symbols seeded from existing broker_symbols.

-- 1. 平台级 canonical 字典 (治理表)
CREATE TABLE IF NOT EXISTS canonical_symbols (
    canonical       text PRIMARY KEY,
    asset_class     text NOT NULL,
    base_ccy        text NOT NULL,
    quote_ccy       text NOT NULL,
    description     text NOT NULL,
    enabled         boolean NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now()
);

-- 2. 租户级 canonical 白名单 (合规边界)
CREATE TABLE IF NOT EXISTS tenant_canonical_whitelist (
    tenant_id   uuid NOT NULL,
    canonical   text NOT NULL REFERENCES canonical_symbols(canonical),
    enabled     boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, canonical)
);
ALTER TABLE tenant_canonical_whitelist ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON tenant_canonical_whitelist
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- 3. 策略级 canonical 白名单 (用户编辑)
CREATE TABLE IF NOT EXISTS strategy_symbols (
    strategy_id uuid NOT NULL REFERENCES strategies(id) ON DELETE CASCADE,
    canonical   text NOT NULL REFERENCES canonical_symbols(canonical),
    enabled     boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (strategy_id, canonical)
);
ALTER TABLE strategy_symbols ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON strategy_symbols
    USING (EXISTS (
        SELECT 1 FROM strategies s
        WHERE s.id = strategy_symbols.strategy_id
        AND s.tenant_id = current_setting('app.tenant_id', true)::uuid
    ));

-- 4. NOTIFY trigger for hot reload (strategy_symbols changes)
CREATE OR REPLACE FUNCTION notify_strategy_symbols() RETURNS trigger AS $$
BEGIN
    PERFORM pg_notify('strategy_symbols', NEW.strategy_id::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_strategy_symbols_notify
    AFTER INSERT OR UPDATE OR DELETE ON strategy_symbols
    FOR EACH ROW EXECUTE FUNCTION notify_strategy_symbols();

-- 5. Seed canonical_symbols from ALL existing broker_symbols data (including trade_mode=0)
INSERT INTO canonical_symbols (canonical, asset_class, base_ccy, quote_ccy, description)
SELECT DISTINCT ON (bs.canonical)
    bs.canonical,
    CASE
        WHEN bs.canonical LIKE 'XAU%' OR bs.canonical LIKE 'XAG%' OR bs.canonical LIKE 'XPT%' OR bs.canonical LIKE 'XPD%' THEN 'metal'
        WHEN bs.canonical LIKE '%USD' AND LENGTH(bs.canonical) = 6 AND bs.canonical ~ '^[A-Z]{6}$'
             AND bs.canonical NOT LIKE 'X%' AND bs.canonical NOT LIKE 'US%' THEN 'forex'
        WHEN bs.canonical LIKE '%USD' AND bs.canonical ~ '^(BTC|ETH|LTC|XRP|SOL|DOGE|ADA|DOT|LINK|UNI|BCH)'
             THEN 'crypto'
        WHEN bs.canonical IN ('US30','US100','US500','GER40','UK100','JPN225','AUS200')
             THEN 'index'
        WHEN bs.canonical IN ('XNGUSD','UKOIL','USOIL') THEN 'commodity'
        ELSE 'forex'
    END,
    LEFT(bs.canonical, LENGTH(bs.canonical)/2),
    RIGHT(bs.canonical, LENGTH(bs.canonical)/2),
    bs.canonical
FROM broker_symbols bs
WHERE bs.canonical IS NOT NULL AND bs.canonical != ''
ORDER BY bs.canonical;

-- 6. Add FK from broker_symbols.canonical → canonical_symbols (NOT VALID first, then validate)
ALTER TABLE broker_symbols
    ADD CONSTRAINT fk_broker_symbols_canonical
    FOREIGN KEY (canonical) REFERENCES canonical_symbols(canonical)
    NOT VALID;
ALTER TABLE broker_symbols VALIDATE CONSTRAINT fk_broker_symbols_canonical;
