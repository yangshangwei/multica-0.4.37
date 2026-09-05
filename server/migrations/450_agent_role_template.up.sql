-- Role-template provenance and autonomy policy on agent.
--
-- A template-created agent is an ordinary workspace agent: its instructions are
-- COPIED into the row at creation so a workspace admin can edit them, and a
-- later template release can never overwrite that edit. These columns record
-- which template the copy came from and at which version, which is what makes a
-- future upgrade offer a diff instead of a silent overwrite.
--
-- autonomy_level is the enforceable half of the role: it is read on every
-- agent-actor request that mutates workspace state, and injected into the
-- claimed task's brief. Empty string means "no policy declared" — every agent
-- that existed before this migration, which must keep behaving exactly as it
-- did (contributor-equivalent, unchanged by this feature).
ALTER TABLE agent
    ADD COLUMN IF NOT EXISTS template_key TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS template_version INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS autonomy_level TEXT NOT NULL DEFAULT '';
