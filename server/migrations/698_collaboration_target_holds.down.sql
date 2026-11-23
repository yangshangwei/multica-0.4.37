DO $$
BEGIN
    LOCK TABLE collaboration_target IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM collaboration_target) THEN
        RAISE EXCEPTION 'cannot remove retained collaboration target hold state';
    END IF;
    ALTER TABLE collaboration_target DROP COLUMN IF EXISTS hold_reason, DROP COLUMN IF EXISTS hold_version;
END;
$$;
