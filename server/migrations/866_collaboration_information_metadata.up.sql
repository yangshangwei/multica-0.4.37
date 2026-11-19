ALTER TABLE issue_handoff
    ADD COLUMN IF NOT EXISTS unblock_condition text,
    ADD COLUMN IF NOT EXISTS estimated_resolution_at timestamptz,
    ADD COLUMN IF NOT EXISTS superseded_by_repair_handoff_id uuid;
