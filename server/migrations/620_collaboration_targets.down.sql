-- Stop every writer for the entire rollback, including account reconciliation.
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
    DROP TRIGGER collaboration_target_identity ON collaboration_target;
    DROP TRIGGER collaboration_command_identity ON collaboration_command;
    DROP TRIGGER collaboration_grant_identity ON collaboration_grant;
    DROP TRIGGER collaboration_group_identity ON collaboration_execution_group;
    DROP TRIGGER collaboration_run_identity ON collaboration_run;
    DROP TRIGGER collaboration_delegation_identity ON collaboration_delegation;
    DROP FUNCTION collaboration_preserve_execution_identity();
    ALTER TABLE collaboration_command DROP CONSTRAINT collaboration_command_target_identity;
    ALTER TABLE collaboration_command DROP COLUMN session_id, DROP COLUMN target_id, DROP COLUMN input_event_id, DROP COLUMN legacy_unresolved;
    ALTER TABLE collaboration_command RENAME COLUMN legacy_session_id TO session_id;
    ALTER TABLE collaboration_command ALTER COLUMN session_id SET NOT NULL;
    ALTER TABLE collaboration_run DROP CONSTRAINT collaboration_run_target_identity, DROP CONSTRAINT collaboration_run_parent_role;
    ALTER TABLE collaboration_run DROP COLUMN session_id, DROP COLUMN command_id, DROP COLUMN target_id, DROP COLUMN group_id,
        DROP COLUMN parent_run_id, DROP COLUMN step_key, DROP COLUMN step_kind, DROP COLUMN agent_id,
        DROP COLUMN target_epoch, DROP COLUMN config_version, DROP COLUMN legacy_unresolved;
    ALTER TABLE collaboration_run RENAME COLUMN legacy_session_id TO session_id;
    ALTER TABLE collaboration_run RENAME COLUMN legacy_command_id TO command_id;
    ALTER TABLE collaboration_run ALTER COLUMN session_id SET NOT NULL, ALTER COLUMN command_id SET NOT NULL;
    ALTER TABLE collaboration_grant DROP CONSTRAINT collaboration_grant_target_identity;
    ALTER TABLE collaboration_grant DROP COLUMN target_id, DROP COLUMN config_version, DROP COLUMN role, DROP COLUMN legacy_unresolved;
    ALTER TABLE collaboration_session DROP COLUMN target_set_version;
    DROP TABLE collaboration_delegation;
    DROP TABLE collaboration_execution_group;
    DROP TABLE collaboration_target;
END $$;
