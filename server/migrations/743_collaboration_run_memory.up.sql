-- Whether the claiming runtime declared shared-context support, and the version
-- it started from. NULL mode is a run created before shared context existed;
-- base version 0 is the empty document. Neither is a runtime capability.
ALTER TABLE collaboration_run
    ADD COLUMN IF NOT EXISTS memory_mode text CHECK (memory_mode IN ('supported', 'unsupported')),
    ADD COLUMN IF NOT EXISTS memory_base_version bigint CHECK (memory_base_version >= 0);
