-- This gate runs before supporting columns are removed by the lower migration.
DO $$ BEGIN
    LOCK TABLE issue_handoff, collaboration_event IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM issue_handoff WHERE unblock_condition IS NOT NULL
        OR estimated_resolution_at IS NOT NULL OR superseded_by_repair_handoff_id IS NOT NULL)
        OR EXISTS (SELECT 1 FROM collaboration_event WHERE kind = 'information_repaired') THEN
        RAISE EXCEPTION 'information recovery history is retained' USING ERRCODE = '23514';
    END IF;
END $$;
