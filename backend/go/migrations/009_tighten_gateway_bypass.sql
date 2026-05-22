-- 009_tighten_gateway_bypass.sql
-- Tighten accounts gateway_bypass RLS policy so only infrastructure services
-- running with app.role='gateway' can read cross-tenant.
-- broker_symbols gateway_bypass is intentionally left open (cross-tenant metadata is OK).

DO $$
BEGIN
    -- Drop the old permissive policy on accounts
    DROP POLICY IF EXISTS gateway_bypass ON accounts;

    -- Recreate with tightened USING clause: only gateway role + empty tenant
    CREATE POLICY gateway_bypass ON accounts
        FOR SELECT
        USING (current_setting('app.role', true) = 'gateway'
           AND current_setting('app.tenant_id', true) = '');

    -- broker_symbols: keep existing gateway_bypass (cross-tenant metadata is OK).
    -- No change needed.
END $$;
