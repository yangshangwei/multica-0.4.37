DO $$
BEGIN
    LOCK TABLE collaboration_config_invalidation IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM collaboration_config_invalidation) THEN
        RAISE EXCEPTION 'cannot remove retained collaboration configuration invalidation identity';
    END IF;
    DROP INDEX IF EXISTS idx_collaboration_config_invalidation_id;
END;
$$;
