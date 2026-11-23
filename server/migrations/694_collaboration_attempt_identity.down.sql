DO $$
BEGIN
    LOCK TABLE collaboration_attempt, collaboration_batch, collaboration_business_capability, collaboration_channel,
        collaboration_command, collaboration_execution_group, collaboration_run, collaboration_runtime_reservation, collaboration_target IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM collaboration_channel) OR EXISTS (SELECT 1 FROM collaboration_batch)
        OR EXISTS (SELECT 1 FROM collaboration_runtime_reservation) OR EXISTS (SELECT 1 FROM collaboration_attempt)
        OR EXISTS (SELECT 1 FROM collaboration_business_capability) THEN
        RAISE EXCEPTION 'cannot remove retained collaboration batch or runtime indexes';
    END IF;
    DROP INDEX idx_collaboration_attempt_identity;
END;
$$;

