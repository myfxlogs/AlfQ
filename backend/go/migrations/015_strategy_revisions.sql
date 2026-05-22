-- 015: Immutable strategy revision snapshots (RS02).
-- Every spec change creates a new revision; old revisions are read-only.
-- All business entities reference revision_id, not strategy_id directly.
CREATE TABLE IF NOT EXISTS strategy_revisions (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    strategy_id     uuid NOT NULL REFERENCES strategies(id) ON DELETE CASCADE,
    revision_no     int NOT NULL,
    spec            jsonb NOT NULL,
    spec_hash       text NOT NULL,          -- SHA-256 of canonical spec JSON
    created_by      uuid REFERENCES users(id),
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE(strategy_id, revision_no)
);

ALTER TABLE strategies ADD COLUMN IF NOT EXISTS current_revision_id uuid REFERENCES strategy_revisions(id);
ALTER TABLE strategies ADD COLUMN IF NOT EXISTS revision_counter int NOT NULL DEFAULT 0;

-- Backfill: create revision #1 for existing strategies with spec
INSERT INTO strategy_revisions (strategy_id, revision_no, spec, spec_hash)
SELECT id, 1, spec, encode(sha256(spec::text::bytea), 'hex')
FROM strategies
WHERE spec IS NOT NULL AND spec::text <> '{}' AND spec::text <> 'null'
  AND NOT EXISTS (SELECT 1 FROM strategy_revisions WHERE strategy_id = strategies.id AND revision_no = 1);

-- Link current_revision_id for backfilled rows
UPDATE strategies
SET current_revision_id = sr.id,
    revision_counter = 1
FROM strategy_revisions sr
WHERE sr.strategy_id = strategies.id AND sr.revision_no = 1
  AND strategies.current_revision_id IS NULL;
