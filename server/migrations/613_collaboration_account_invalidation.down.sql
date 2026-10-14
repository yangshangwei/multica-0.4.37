DO $$
BEGIN
    LOCK TABLE collaboration_account_invalidation IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM collaboration_account_invalidation) THEN
        RAISE EXCEPTION 'cannot roll back retained collaboration account invalidation';
    END IF;
    DROP TABLE collaboration_account_invalidation;
END $$;
