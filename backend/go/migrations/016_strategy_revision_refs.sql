-- 016: backtest/order tables reference strategy_revisions (RS02).
-- Ensures audit lineage: every backtest and order is pinned to the exact spec revision.

ALTER TABLE backtest_results ADD COLUMN IF NOT EXISTS strategy_revision_id uuid REFERENCES strategy_revisions(id);
ALTER TABLE orders ADD COLUMN IF NOT EXISTS strategy_revision_id uuid REFERENCES strategy_revisions(id);

-- Backfill: set strategy_revision_id = current_revision_id for existing rows
UPDATE backtest_results br
SET strategy_revision_id = s.current_revision_id
FROM strategies s
WHERE br.strategy_id = s.id
  AND br.strategy_revision_id IS NULL
  AND s.current_revision_id IS NOT NULL;

UPDATE orders o
SET strategy_revision_id = s.current_revision_id
FROM strategies s
WHERE o.strategy_id = s.id
  AND o.strategy_revision_id IS NULL
  AND s.current_revision_id IS NOT NULL;
