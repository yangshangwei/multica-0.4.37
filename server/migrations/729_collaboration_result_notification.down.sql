DO $$
BEGIN
    LOCK TABLE collaboration_notification IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM collaboration_notification WHERE run_id IS NOT NULL) THEN
        RAISE EXCEPTION 'retained collaboration result notifications prevent rollback';
    END IF;
    ALTER TABLE collaboration_notification DROP COLUMN IF EXISTS run_id;
END $$;
