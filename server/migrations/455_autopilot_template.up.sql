-- Autopilot-template provenance.
--
-- A template-created autopilot is an ordinary workspace autopilot: the
-- template's prompt, cron and execution mode are COPIED into the row and its
-- schedule trigger at creation, so the owner can edit them afterwards and a
-- later template release can never overwrite that edit. These columns record
-- which template the copy came from and at which version, which is what makes a
-- future upgrade offer a diff instead of a silent overwrite.
--
-- Empty template_key with version 0 means "not from a template" — every
-- autopilot created by hand, including every row that existed before this
-- migration, which must keep behaving exactly as it did.
ALTER TABLE autopilot
    ADD COLUMN IF NOT EXISTS template_key TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS template_version INTEGER NOT NULL DEFAULT 0;
