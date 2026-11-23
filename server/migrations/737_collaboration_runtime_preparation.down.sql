DO $$
BEGIN
    LOCK TABLE agent_runtime, collaboration_run, collaboration_run_event IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM agent_runtime WHERE collaboration_capabilities IS NOT NULL)
        OR EXISTS (SELECT 1 FROM collaboration_run WHERE preparation_snapshot IS NOT NULL)
        OR EXISTS (SELECT 1 FROM collaboration_run_event WHERE kind = 'preparation') THEN
        RAISE EXCEPTION 'cannot remove retained collaboration preparation evidence';
    END IF;
    DROP TRIGGER collaboration_run_preparation_identity ON collaboration_run;
    DROP FUNCTION collaboration_preserve_run_preparation();
    DROP TRIGGER collaboration_runtime_capability_epoch ON agent_runtime;
    DROP FUNCTION collaboration_prepare_capability_epoch();
    ALTER TABLE collaboration_run DROP COLUMN preparation_snapshot;
    ALTER TABLE agent_runtime DROP COLUMN collaboration_capabilities;
END $$;
