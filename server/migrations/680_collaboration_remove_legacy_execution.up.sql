-- The unreleased single-session execution path is intentionally removed (M10).
-- Current typed inputs/runs and all human/account/content history are retained.
BEGIN;
LOCK TABLE collaboration_command, collaboration_delegation, collaboration_execution_group,
    collaboration_grant, collaboration_run, collaboration_run_event, collaboration_session,
    collaboration_target, collaboration_termination IN ACCESS EXCLUSIVE MODE NOWAIT;

DELETE FROM collaboration_run_event e USING collaboration_run r WHERE e.run_id = r.id AND r.legacy_unresolved;
DELETE FROM collaboration_termination t USING collaboration_run r WHERE t.run_id = r.id AND r.legacy_unresolved;
DELETE FROM collaboration_run WHERE legacy_unresolved;
DELETE FROM collaboration_command WHERE legacy_unresolved;
DELETE FROM collaboration_grant WHERE legacy_unresolved;

ALTER TABLE collaboration_command DROP CONSTRAINT collaboration_command_target_identity;
ALTER TABLE collaboration_command DROP COLUMN legacy_session_id, DROP COLUMN legacy_unresolved;
ALTER TABLE collaboration_command ADD CONSTRAINT collaboration_command_target_identity CHECK (target_id IS NOT NULL AND input_event_id IS NOT NULL);

ALTER TABLE collaboration_grant DROP CONSTRAINT collaboration_grant_target_identity;
ALTER TABLE collaboration_grant DROP COLUMN legacy_unresolved;
ALTER TABLE collaboration_grant ADD CONSTRAINT collaboration_grant_target_identity CHECK (target_id IS NOT NULL AND config_version IS NOT NULL AND role IS NOT NULL);

ALTER TABLE collaboration_run DROP CONSTRAINT collaboration_run_target_identity, DROP CONSTRAINT collaboration_run_parent_role;
ALTER TABLE collaboration_run DROP COLUMN legacy_session_id, DROP COLUMN legacy_command_id, DROP COLUMN legacy_unresolved;
ALTER TABLE collaboration_run ADD CONSTRAINT collaboration_run_target_identity CHECK (
    target_id IS NOT NULL AND group_id IS NOT NULL AND step_key IS NOT NULL AND step_kind IS NOT NULL
    AND agent_id IS NOT NULL AND target_epoch IS NOT NULL AND config_version IS NOT NULL
);
ALTER TABLE collaboration_run ADD CONSTRAINT collaboration_run_parent_role CHECK (
    (step_kind IN ('agent', 'leader') AND parent_run_id IS NULL) OR
    (step_kind IN ('member', 'continuation') AND parent_run_id IS NOT NULL AND parent_run_id <> id)
);
ALTER TABLE collaboration_session DROP COLUMN active_run_id, DROP COLUMN active_grant_id,
    DROP COLUMN next_command_seq, DROP COLUMN resource_epoch;

CREATE OR REPLACE FUNCTION collaboration_preserve_execution_identity() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_TABLE_NAME = 'collaboration_target' THEN
        IF TG_OP = 'UPDATE' AND ROW(NEW.id, NEW.session_id, NEW.kind, NEW.resource_id)
            IS DISTINCT FROM ROW(OLD.id, OLD.session_id, OLD.kind, OLD.resource_id) THEN
            RAISE EXCEPTION 'collaboration target identity is immutable' USING ERRCODE = '23514';
        END IF;
        IF TG_OP = 'UPDATE' AND NEW.roster IS DISTINCT FROM OLD.roster AND NEW.config_version <= OLD.config_version THEN
            RAISE EXCEPTION 'collaboration roster requires a new configuration' USING ERRCODE = '23514';
        END IF;
    ELSIF TG_TABLE_NAME = 'collaboration_command' THEN
        IF TG_OP = 'UPDATE' THEN
            IF ROW(NEW.id, NEW.session_id, NEW.target_id, NEW.input_event_id, NEW.seq, NEW.author_user_id,
                NEW.participant_epoch, NEW.author_auth_epoch, NEW.lifecycle_epoch, NEW.content, NEW.attachments)
                IS DISTINCT FROM ROW(OLD.id, OLD.session_id, OLD.target_id, OLD.input_event_id, OLD.seq, OLD.author_user_id,
                OLD.participant_epoch, OLD.author_auth_epoch, OLD.lifecycle_epoch, OLD.content, OLD.attachments) THEN
                RAISE EXCEPTION 'collaboration command identity is immutable' USING ERRCODE = '23514';
            END IF;
            IF NEW.state IS DISTINCT FROM OLD.state AND NOT
                ((OLD.state = 'waiting' AND NEW.state IN ('in_flight', 'cancelled')) OR
                 (OLD.state = 'in_flight' AND NEW.state IN ('succeeded', 'failed', 'stopped', 'cancelled'))) THEN
                RAISE EXCEPTION 'collaboration command cannot be revived' USING ERRCODE = '23514';
            END IF;
        END IF;
    ELSIF TG_TABLE_NAME = 'collaboration_grant' THEN
        IF TG_OP = 'UPDATE' AND ROW(NEW.id, NEW.issue_id, NEW.target_id, NEW.config_version, NEW.role, NEW.owner_user_id,
            NEW.owner_auth_epoch, NEW.execution_workspace_id, NEW.agent_id, NEW.runtime_id, NEW.identity_kind,
            NEW.installation_id, NEW.binding_id, NEW.binding_epoch)
            IS DISTINCT FROM ROW(OLD.id, OLD.issue_id, OLD.target_id, OLD.config_version, OLD.role, OLD.owner_user_id,
            OLD.owner_auth_epoch, OLD.execution_workspace_id, OLD.agent_id, OLD.runtime_id, OLD.identity_kind,
            OLD.installation_id, OLD.binding_id, OLD.binding_epoch) THEN
            RAISE EXCEPTION 'collaboration grant identity is immutable' USING ERRCODE = '23514';
        END IF;
    ELSIF TG_TABLE_NAME = 'collaboration_execution_group' THEN
        IF TG_OP = 'UPDATE' THEN
            IF ROW(NEW.id, NEW.session_id, NEW.target_id, NEW.command_id, NEW.generation, NEW.target_epoch, NEW.config_version, NEW.roster)
                IS DISTINCT FROM ROW(OLD.id, OLD.session_id, OLD.target_id, OLD.command_id, OLD.generation, OLD.target_epoch, OLD.config_version, OLD.roster) THEN
                RAISE EXCEPTION 'collaboration group identity is immutable' USING ERRCODE = '23514';
            END IF;
            IF (OLD.expansion_sealed_at IS NOT NULL AND NEW.expansion_sealed_at IS DISTINCT FROM OLD.expansion_sealed_at)
                OR (OLD.settled_at IS NOT NULL AND ROW(NEW.settled_at, NEW.state) IS DISTINCT FROM ROW(OLD.settled_at, OLD.state))
                OR (OLD.forced_disposition IS NOT NULL AND ROW(NEW.forced_disposition, NEW.disposition_reason) IS DISTINCT FROM ROW(OLD.forced_disposition, OLD.disposition_reason)) THEN
                RAISE EXCEPTION 'collaboration group cannot be reopened or redisposed' USING ERRCODE = '23514';
            END IF;
        END IF;
    ELSIF TG_TABLE_NAME = 'collaboration_run' THEN
        IF TG_OP = 'UPDATE' AND ROW(NEW.id, NEW.session_id, NEW.command_id, NEW.target_id, NEW.group_id, NEW.parent_run_id,
            NEW.step_key, NEW.step_kind, NEW.agent_id, NEW.target_epoch, NEW.config_version, NEW.grant_id, NEW.grant_epoch,
            NEW.resource_epoch, NEW.execution_workspace_id, NEW.runtime_id, NEW.installation_id, NEW.binding_id, NEW.binding_epoch, NEW.fence, NEW.launch_nonce)
            IS DISTINCT FROM ROW(OLD.id, OLD.session_id, OLD.command_id, OLD.target_id, OLD.group_id, OLD.parent_run_id,
            OLD.step_key, OLD.step_kind, OLD.agent_id, OLD.target_epoch, OLD.config_version, OLD.grant_id, OLD.grant_epoch,
            OLD.resource_epoch, OLD.execution_workspace_id, OLD.runtime_id, OLD.installation_id, OLD.binding_id, OLD.binding_epoch, OLD.fence, OLD.launch_nonce) THEN
            RAISE EXCEPTION 'collaboration run identity is immutable' USING ERRCODE = '23514';
        END IF;
    ELSIF TG_TABLE_NAME = 'collaboration_delegation' THEN
        IF TG_OP = 'UPDATE' AND ROW(NEW.id, NEW.group_id, NEW.parent_run_id, NEW.request_key, NEW.payload_hash, NEW.phase_key, NEW.members, NEW.continuation_key)
            IS DISTINCT FROM ROW(OLD.id, OLD.group_id, OLD.parent_run_id, OLD.request_key, OLD.payload_hash, OLD.phase_key, OLD.members, OLD.continuation_key) THEN
            RAISE EXCEPTION 'collaboration delegation identity is immutable' USING ERRCODE = '23514';
        END IF;
    END IF;
    RETURN NEW;
END;
$$;
COMMIT;
