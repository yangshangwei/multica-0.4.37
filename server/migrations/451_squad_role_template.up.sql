-- Squad-template provenance. Same contract as agent.template_key: the squad's
-- leader instructions are copied at creation and stay workspace-owned, and
-- these columns only record where the copy came from.
ALTER TABLE squad
    ADD COLUMN IF NOT EXISTS template_key TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS template_version INTEGER NOT NULL DEFAULT 0;
