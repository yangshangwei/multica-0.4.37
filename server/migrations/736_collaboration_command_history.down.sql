DO $$
BEGIN
    LOCK TABLE collaboration_command IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM collaboration_command WHERE cancellation_event_id IS NOT NULL) THEN
        RAISE EXCEPTION 'cannot remove retained collaboration command history index';
    END IF;
    DROP INDEX collaboration_command_history_idx;
END $$;
