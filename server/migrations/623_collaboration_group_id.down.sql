DO $$
BEGIN
    LOCK TABLE collaboration_command, collaboration_delegation, collaboration_execution_gate,
        collaboration_execution_group, collaboration_grant, collaboration_run,
        collaboration_session, collaboration_target, collaboration_termination
        IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM collaboration_command) OR EXISTS (SELECT 1 FROM collaboration_delegation)
       OR EXISTS (SELECT 1 FROM collaboration_execution_group) OR EXISTS (SELECT 1 FROM collaboration_grant)
       OR EXISTS (SELECT 1 FROM collaboration_run) OR EXISTS (SELECT 1 FROM collaboration_session)
       OR EXISTS (SELECT 1 FROM collaboration_target) OR EXISTS (SELECT 1 FROM collaboration_termination)
       OR EXISTS (SELECT 1 FROM collaboration_execution_gate WHERE enabled OR creation_enabled) THEN
        RAISE EXCEPTION 'cannot roll back retained collaboration target state';
    END IF;
    DROP INDEX idx_collaboration_group_id;
END $$;
