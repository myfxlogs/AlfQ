-- 018: strategy_revisions NOTIFY trigger for hot-reload (RS05).
-- Quant-engine LISTENs on 'strategy_revisions' channel.
-- When a new revision is created, the engine gets notified and creates a new Runtime.

CREATE OR REPLACE FUNCTION notify_strategy_revision()
RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('strategy_revisions', NEW.id::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_strategy_revisions_notify ON strategy_revisions;
CREATE TRIGGER trg_strategy_revisions_notify
    AFTER INSERT ON strategy_revisions
    FOR EACH ROW
    EXECUTE FUNCTION notify_strategy_revision();
