DO $$
BEGIN
    LOCK TABLE collaboration_notification IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM collaboration_notification WHERE source_kind <> 'issue' OR source_id IS NOT NULL) THEN
        RAISE EXCEPTION 'collaboration notification sources are retained; rollback refused';
    END IF;
    ALTER TABLE collaboration_notification DROP COLUMN source_kind, DROP COLUMN source_id;
END $$;
