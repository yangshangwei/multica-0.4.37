DO $$
BEGIN
    LOCK TABLE collaboration_account_invalidation IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM collaboration_account_invalidation) THEN
        RAISE EXCEPTION 'cannot roll back retained collaboration account invalidation progress';
    END IF;
    ALTER TABLE collaboration_account_invalidation DROP COLUMN last_issue_id;
END $$;
