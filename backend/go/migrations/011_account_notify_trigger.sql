-- 011_account_notify_trigger.sql
-- Creates a PG NOTIFY trigger so md-gateway can detect new/updated accounts
-- without polling. Fires on INSERT and UPDATE of status/disabling.

CREATE OR REPLACE FUNCTION notify_account_change() RETURNS trigger AS $$
BEGIN
    PERFORM pg_notify('account_changes', NEW.id::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_account_change ON accounts;
CREATE TRIGGER trg_account_change
    AFTER INSERT OR UPDATE OF status, is_disabled ON accounts
    FOR EACH ROW EXECUTE FUNCTION notify_account_change();
