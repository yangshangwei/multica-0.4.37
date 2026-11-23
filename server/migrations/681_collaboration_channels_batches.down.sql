DO $$
BEGIN
    LOCK TABLE collaboration_attempt, collaboration_batch, collaboration_business_capability, collaboration_channel,
        collaboration_command, collaboration_execution_group, collaboration_run, collaboration_runtime_reservation, collaboration_target IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM collaboration_batch) OR EXISTS (SELECT 1 FROM collaboration_channel)
        OR EXISTS (SELECT 1 FROM collaboration_runtime_reservation) OR EXISTS (SELECT 1 FROM collaboration_attempt)
        OR EXISTS (SELECT 1 FROM collaboration_business_capability) THEN
        RAISE EXCEPTION 'cannot remove retained collaboration batch or runtime state';
    END IF;
    DROP TRIGGER collaboration_command_batch_identity ON collaboration_command;
    DROP TRIGGER collaboration_group_batch_identity ON collaboration_execution_group;
    DROP TRIGGER collaboration_run_batch_identity ON collaboration_run;
    DROP FUNCTION collaboration_preserve_batch_input();
    ALTER TABLE collaboration_target DROP COLUMN channel_id;
    ALTER TABLE collaboration_command DROP COLUMN channel_id,DROP COLUMN batch_id;
    ALTER TABLE collaboration_execution_group DROP COLUMN channel_id,DROP COLUMN batch_id;
    ALTER TABLE collaboration_run DROP COLUMN channel_id,DROP COLUMN batch_id;
    DROP TABLE collaboration_attempt,collaboration_business_capability,collaboration_runtime_reservation,collaboration_batch,collaboration_channel;
END;
$$;
