DO $$
BEGIN
    LOCK TABLE collaboration_command, collaboration_event IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM collaboration_command WHERE cancellation_event_id IS NOT NULL) THEN
        RAISE EXCEPTION 'cannot remove retained collaboration cancellation provenance';
    END IF;
    DROP TRIGGER collaboration_cancellation_event_identity ON collaboration_event;
    DROP FUNCTION collaboration_preserve_cancellation_event();
    DROP TRIGGER collaboration_command_cancellation_identity ON collaboration_command;
    DROP FUNCTION collaboration_preserve_command_cancellation();
    ALTER TABLE collaboration_command DROP COLUMN cancellation_event_id;
END $$;
