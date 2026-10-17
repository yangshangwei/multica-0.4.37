-- Keep the old index keys as retained legacy identities. New typed rows never
-- write them, so the original constraints remain available for an empty rollback.
-- All new indexes are built concurrently by the following migrations.
BEGIN;

ALTER TABLE collaboration_session ADD COLUMN target_set_version bigint NOT NULL DEFAULT 1 CHECK (target_set_version > 0);

CREATE TABLE collaboration_target (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL,
    kind text NOT NULL CHECK (kind IN ('agent', 'squad')),
    resource_id uuid NOT NULL,
    target_epoch bigint NOT NULL DEFAULT 1 CHECK (target_epoch > 0),
    config_version bigint NOT NULL DEFAULT 1 CHECK (config_version > 0),
    roster jsonb NOT NULL CHECK (jsonb_typeof(roster) = 'array'),
    queue_state text NOT NULL DEFAULT 'paused' CHECK (queue_state IN ('paused', 'active')),
    control_version bigint NOT NULL DEFAULT 1 CHECK (control_version > 0),
    group_generation bigint NOT NULL DEFAULT 0 CHECK (group_generation >= 0),
    next_command_seq bigint NOT NULL DEFAULT 1 CHECK (next_command_seq > 0),
    active_group_id uuid,
    pause_reason text NOT NULL DEFAULT 'new_target',
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE collaboration_execution_group (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL,
    target_id uuid NOT NULL,
    command_id uuid NOT NULL,
    generation bigint NOT NULL CHECK (generation > 0),
    target_epoch bigint NOT NULL CHECK (target_epoch > 0),
    config_version bigint NOT NULL CHECK (config_version > 0),
    roster jsonb NOT NULL CHECK (jsonb_typeof(roster) = 'array'),
    state text NOT NULL DEFAULT 'active' CHECK (state IN ('active', 'waiting_children', 'ending', 'succeeded', 'failed', 'stopped', 'cancelled')),
    expansion_sealed_at timestamptz,
    forced_disposition text CHECK (forced_disposition IN ('stopped', 'cancelled')),
    disposition_reason text,
    created_at timestamptz NOT NULL DEFAULT now(),
    settled_at timestamptz,
    CHECK ((settled_at IS NULL AND state IN ('active', 'waiting_children', 'ending')) OR
           (settled_at IS NOT NULL AND expansion_sealed_at IS NOT NULL AND state IN ('succeeded', 'failed', 'stopped', 'cancelled')))
);

CREATE TABLE collaboration_delegation (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    group_id uuid NOT NULL,
    parent_run_id uuid NOT NULL,
    request_key text NOT NULL CHECK (length(request_key) BETWEEN 1 AND 128),
    payload_hash text NOT NULL,
    phase_key text NOT NULL CHECK (length(phase_key) BETWEEN 1 AND 128),
    members jsonb NOT NULL CHECK (jsonb_typeof(members) = 'array'),
    continuation_key text NOT NULL CHECK (length(continuation_key) BETWEEN 1 AND 128),
    state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'waiting', 'settled', 'cancelled')),
    disposition_reason text,
    created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE collaboration_command RENAME COLUMN session_id TO legacy_session_id;
ALTER TABLE collaboration_command ALTER COLUMN legacy_session_id DROP NOT NULL;
ALTER TABLE collaboration_command ADD COLUMN session_id uuid;
ALTER TABLE collaboration_command ADD COLUMN target_id uuid;
ALTER TABLE collaboration_command ADD COLUMN input_event_id uuid;
ALTER TABLE collaboration_command ADD COLUMN legacy_unresolved boolean NOT NULL DEFAULT false;
UPDATE collaboration_command SET session_id = legacy_session_id, legacy_unresolved = true;
ALTER TABLE collaboration_command ALTER COLUMN session_id SET NOT NULL;
ALTER TABLE collaboration_command ADD CONSTRAINT collaboration_command_target_identity CHECK (
    (legacy_unresolved AND target_id IS NULL AND input_event_id IS NULL AND legacy_session_id IS NOT NULL) OR
    (NOT legacy_unresolved AND target_id IS NOT NULL AND input_event_id IS NOT NULL AND legacy_session_id IS NULL)
);

ALTER TABLE collaboration_grant ADD COLUMN target_id uuid;
ALTER TABLE collaboration_grant ADD COLUMN config_version bigint CHECK (config_version > 0);
ALTER TABLE collaboration_grant ADD COLUMN role text CHECK (role IN ('agent', 'leader', 'member'));
ALTER TABLE collaboration_grant ADD COLUMN legacy_unresolved boolean NOT NULL DEFAULT false;
UPDATE collaboration_grant SET legacy_unresolved = true;
ALTER TABLE collaboration_grant ADD CONSTRAINT collaboration_grant_target_identity CHECK (
    (legacy_unresolved AND target_id IS NULL AND config_version IS NULL AND role IS NULL) OR
    (NOT legacy_unresolved AND target_id IS NOT NULL AND config_version IS NOT NULL AND role IS NOT NULL)
);

ALTER TABLE collaboration_run RENAME COLUMN session_id TO legacy_session_id;
ALTER TABLE collaboration_run ALTER COLUMN legacy_session_id DROP NOT NULL;
ALTER TABLE collaboration_run RENAME COLUMN command_id TO legacy_command_id;
ALTER TABLE collaboration_run ALTER COLUMN legacy_command_id DROP NOT NULL;
ALTER TABLE collaboration_run ADD COLUMN session_id uuid;
ALTER TABLE collaboration_run ADD COLUMN command_id uuid;
ALTER TABLE collaboration_run ADD COLUMN target_id uuid;
ALTER TABLE collaboration_run ADD COLUMN group_id uuid;
ALTER TABLE collaboration_run ADD COLUMN parent_run_id uuid;
ALTER TABLE collaboration_run ADD COLUMN step_key text CHECK (length(step_key) BETWEEN 1 AND 128);
ALTER TABLE collaboration_run ADD COLUMN step_kind text CHECK (step_kind IN ('agent', 'leader', 'member', 'continuation'));
ALTER TABLE collaboration_run ADD COLUMN agent_id uuid;
ALTER TABLE collaboration_run ADD COLUMN target_epoch bigint CHECK (target_epoch > 0);
ALTER TABLE collaboration_run ADD COLUMN config_version bigint CHECK (config_version > 0);
ALTER TABLE collaboration_run ADD COLUMN legacy_unresolved boolean NOT NULL DEFAULT false;
UPDATE collaboration_run SET session_id = legacy_session_id, command_id = legacy_command_id, legacy_unresolved = true;
ALTER TABLE collaboration_run ALTER COLUMN session_id SET NOT NULL;
ALTER TABLE collaboration_run ALTER COLUMN command_id SET NOT NULL;
ALTER TABLE collaboration_run ADD CONSTRAINT collaboration_run_target_identity CHECK (
    (legacy_unresolved AND target_id IS NULL AND group_id IS NULL AND parent_run_id IS NULL AND step_key IS NULL AND step_kind IS NULL
        AND agent_id IS NULL AND target_epoch IS NULL AND config_version IS NULL AND legacy_session_id IS NOT NULL AND legacy_command_id IS NOT NULL) OR
    (NOT legacy_unresolved AND target_id IS NOT NULL AND group_id IS NOT NULL AND step_key IS NOT NULL AND step_kind IS NOT NULL
        AND agent_id IS NOT NULL AND target_epoch IS NOT NULL AND config_version IS NOT NULL AND legacy_session_id IS NULL AND legacy_command_id IS NULL)
);
ALTER TABLE collaboration_run ADD CONSTRAINT collaboration_run_parent_role CHECK (
    legacy_unresolved OR ((step_kind IN ('agent', 'leader') AND parent_run_id IS NULL) OR
                          (step_kind IN ('member', 'continuation') AND parent_run_id IS NOT NULL AND parent_run_id <> id))
);

-- Identity, snapshots and parent links are immutable. Retained unresolved rows
-- may be cancelled/stopped but can never acquire a guessed target or new permit.
CREATE FUNCTION collaboration_preserve_execution_identity() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_TABLE_NAME = 'collaboration_target' THEN
        IF TG_OP = 'UPDATE' AND (NEW.id, NEW.session_id, NEW.kind, NEW.resource_id) IS DISTINCT FROM (OLD.id, OLD.session_id, OLD.kind, OLD.resource_id) THEN
            RAISE EXCEPTION 'collaboration target identity is immutable' USING ERRCODE = '23514';
        END IF;
        IF TG_OP = 'UPDATE' AND NEW.roster IS DISTINCT FROM OLD.roster AND NEW.config_version <= OLD.config_version THEN
            RAISE EXCEPTION 'collaboration roster requires a new configuration' USING ERRCODE = '23514';
        END IF;
    ELSIF TG_TABLE_NAME = 'collaboration_command' THEN
        IF TG_OP = 'INSERT' AND NEW.legacy_unresolved THEN
            RAISE EXCEPTION 'new collaboration commands require a target' USING ERRCODE = '23514';
        END IF;
        IF TG_OP = 'UPDATE' THEN
            IF (NEW.id, NEW.session_id, NEW.legacy_session_id, NEW.target_id, NEW.input_event_id, NEW.seq, NEW.author_user_id,
                NEW.participant_epoch, NEW.author_auth_epoch, NEW.lifecycle_epoch, NEW.content, NEW.attachments, NEW.legacy_unresolved)
                IS DISTINCT FROM
               (OLD.id, OLD.session_id, OLD.legacy_session_id, OLD.target_id, OLD.input_event_id, OLD.seq, OLD.author_user_id,
                OLD.participant_epoch, OLD.author_auth_epoch, OLD.lifecycle_epoch, OLD.content, OLD.attachments, OLD.legacy_unresolved) THEN
                RAISE EXCEPTION 'collaboration command identity and input are immutable' USING ERRCODE = '23514';
            END IF;
            IF NEW.legacy_unresolved AND NEW.state = 'in_flight' AND OLD.state <> 'in_flight' THEN
                RAISE EXCEPTION 'unresolved collaboration input cannot execute' USING ERRCODE = '23514';
            END IF;
            IF NEW.state IS DISTINCT FROM OLD.state AND NOT
                ((OLD.state = 'waiting' AND NEW.state IN ('in_flight', 'cancelled')) OR
                 (OLD.state = 'in_flight' AND NEW.state IN ('succeeded', 'failed', 'stopped', 'cancelled'))) THEN
                RAISE EXCEPTION 'collaboration command cannot be revived' USING ERRCODE = '23514';
            END IF;
        END IF;
    ELSIF TG_TABLE_NAME = 'collaboration_grant' THEN
        IF TG_OP = 'INSERT' AND NEW.legacy_unresolved THEN
            RAISE EXCEPTION 'new collaboration grants require a target' USING ERRCODE = '23514';
        END IF;
        IF TG_OP = 'UPDATE' AND
           (NEW.id, NEW.issue_id, NEW.target_id, NEW.config_version, NEW.role, NEW.owner_user_id, NEW.owner_auth_epoch,
            NEW.execution_workspace_id, NEW.agent_id, NEW.runtime_id, NEW.identity_kind, NEW.installation_id, NEW.binding_id, NEW.binding_epoch, NEW.legacy_unresolved)
           IS DISTINCT FROM
           (OLD.id, OLD.issue_id, OLD.target_id, OLD.config_version, OLD.role, OLD.owner_user_id, OLD.owner_auth_epoch,
            OLD.execution_workspace_id, OLD.agent_id, OLD.runtime_id, OLD.identity_kind, OLD.installation_id, OLD.binding_id, OLD.binding_epoch, OLD.legacy_unresolved) THEN
            RAISE EXCEPTION 'collaboration grant identity is immutable' USING ERRCODE = '23514';
        END IF;
    ELSIF TG_TABLE_NAME = 'collaboration_execution_group' THEN
        IF TG_OP = 'UPDATE' THEN
            IF (NEW.id, NEW.session_id, NEW.target_id, NEW.command_id, NEW.generation, NEW.target_epoch, NEW.config_version, NEW.roster)
               IS DISTINCT FROM (OLD.id, OLD.session_id, OLD.target_id, OLD.command_id, OLD.generation, OLD.target_epoch, OLD.config_version, OLD.roster) THEN
                RAISE EXCEPTION 'collaboration group identity is immutable' USING ERRCODE = '23514';
            END IF;
            IF (OLD.expansion_sealed_at IS NOT NULL AND NEW.expansion_sealed_at IS DISTINCT FROM OLD.expansion_sealed_at) OR
               (OLD.settled_at IS NOT NULL AND (NEW.settled_at, NEW.state) IS DISTINCT FROM (OLD.settled_at, OLD.state)) OR
               (OLD.forced_disposition IS NOT NULL AND (NEW.forced_disposition, NEW.disposition_reason) IS DISTINCT FROM (OLD.forced_disposition, OLD.disposition_reason)) THEN
                RAISE EXCEPTION 'collaboration group cannot be reopened or redisposed' USING ERRCODE = '23514';
            END IF;
        END IF;
    ELSIF TG_TABLE_NAME = 'collaboration_run' THEN
        IF TG_OP = 'INSERT' AND NEW.legacy_unresolved THEN
            RAISE EXCEPTION 'new collaboration runs require a target and group' USING ERRCODE = '23514';
        END IF;
        IF TG_OP = 'UPDATE' THEN
            IF (NEW.id, NEW.session_id, NEW.command_id, NEW.legacy_session_id, NEW.legacy_command_id, NEW.target_id, NEW.group_id, NEW.parent_run_id,
                NEW.step_key, NEW.step_kind, NEW.agent_id, NEW.target_epoch, NEW.config_version, NEW.grant_id, NEW.grant_epoch, NEW.resource_epoch,
                NEW.execution_workspace_id, NEW.runtime_id, NEW.installation_id, NEW.binding_id, NEW.binding_epoch, NEW.fence, NEW.launch_nonce, NEW.legacy_unresolved)
                IS DISTINCT FROM
               (OLD.id, OLD.session_id, OLD.command_id, OLD.legacy_session_id, OLD.legacy_command_id, OLD.target_id, OLD.group_id, OLD.parent_run_id,
                OLD.step_key, OLD.step_kind, OLD.agent_id, OLD.target_epoch, OLD.config_version, OLD.grant_id, OLD.grant_epoch, OLD.resource_epoch,
                OLD.execution_workspace_id, OLD.runtime_id, OLD.installation_id, OLD.binding_id, OLD.binding_epoch, OLD.fence, OLD.launch_nonce, OLD.legacy_unresolved) THEN
                RAISE EXCEPTION 'collaboration run identity is immutable' USING ERRCODE = '23514';
            END IF;
            IF NEW.legacy_unresolved AND NEW.authorized_attempt_seq IS DISTINCT FROM OLD.authorized_attempt_seq THEN
                RAISE EXCEPTION 'unresolved collaboration run cannot obtain a permit' USING ERRCODE = '23514';
            END IF;
        END IF;
    ELSIF TG_TABLE_NAME = 'collaboration_delegation' THEN
        IF TG_OP = 'UPDATE' AND
           (NEW.id, NEW.group_id, NEW.parent_run_id, NEW.request_key, NEW.payload_hash, NEW.phase_key, NEW.members, NEW.continuation_key)
           IS DISTINCT FROM
           (OLD.id, OLD.group_id, OLD.parent_run_id, OLD.request_key, OLD.payload_hash, OLD.phase_key, OLD.members, OLD.continuation_key) THEN
            RAISE EXCEPTION 'collaboration delegation identity is immutable' USING ERRCODE = '23514';
        END IF;
    END IF;
    RETURN NEW;
END $$;

CREATE TRIGGER collaboration_target_identity BEFORE INSERT OR UPDATE ON collaboration_target FOR EACH ROW EXECUTE FUNCTION collaboration_preserve_execution_identity();
CREATE TRIGGER collaboration_command_identity BEFORE INSERT OR UPDATE ON collaboration_command FOR EACH ROW EXECUTE FUNCTION collaboration_preserve_execution_identity();
CREATE TRIGGER collaboration_grant_identity BEFORE INSERT OR UPDATE ON collaboration_grant FOR EACH ROW EXECUTE FUNCTION collaboration_preserve_execution_identity();
CREATE TRIGGER collaboration_group_identity BEFORE INSERT OR UPDATE ON collaboration_execution_group FOR EACH ROW EXECUTE FUNCTION collaboration_preserve_execution_identity();
CREATE TRIGGER collaboration_run_identity BEFORE INSERT OR UPDATE ON collaboration_run FOR EACH ROW EXECUTE FUNCTION collaboration_preserve_execution_identity();
CREATE TRIGGER collaboration_delegation_identity BEFORE INSERT OR UPDATE ON collaboration_delegation FOR EACH ROW EXECUTE FUNCTION collaboration_preserve_execution_identity();

-- An old grant does not reveal whether the author intended a direct agent or a
-- squad. Preserve every old key/nonce/proof and require explicit later handling.
UPDATE collaboration_session SET queue_state = CASE WHEN queue_state = 'closed' THEN 'closed' ELSE 'paused' END,
    control_version = control_version + 1, pause_reason = CASE WHEN queue_state = 'active' THEN 'protocol_upgrade' ELSE pause_reason END;
UPDATE collaboration_execution_gate SET enabled = false, creation_enabled = false, epoch = epoch + 1 WHERE id = 1;

COMMIT;
