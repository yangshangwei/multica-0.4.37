BEGIN;
LOCK TABLE collaboration_command, collaboration_execution_group, collaboration_grant,
    collaboration_group_input, collaboration_run, collaboration_target IN ACCESS EXCLUSIVE MODE NOWAIT;

CREATE TABLE collaboration_group_dependency (
    group_id uuid NOT NULL,
    command_id uuid NOT NULL,
    target_id uuid NOT NULL,
    target_epoch bigint NOT NULL CHECK (target_epoch > 0),
    config_version bigint NOT NULL CHECK (config_version > 0),
    configuration_fingerprint text NOT NULL CHECK (configuration_fingerprint <> ''),
    grants jsonb NOT NULL CHECK (jsonb_typeof(grants)='array' AND jsonb_array_length(grants)>0),
    detached_at timestamptz,
    detach_reason text,
    CHECK ((detached_at IS NULL) = (detach_reason IS NULL))
);

-- Released direct-agent groups can be reconstructed from their immutable grant
-- identities. Refuse ambiguous older groups instead of borrowing current consent.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM collaboration_group_input i
        JOIN collaboration_execution_group g ON g.id=i.group_id
        WHERE i.command_id IS NOT NULL AND (
            NOT EXISTS (SELECT 1 FROM collaboration_command c WHERE c.id=i.command_id AND c.target_id=g.target_id)
            OR NOT EXISTS (SELECT 1 FROM collaboration_grant grant_row
                WHERE grant_row.target_id=g.target_id AND grant_row.target_epoch=g.target_epoch
                  AND grant_row.config_version=g.config_version AND grant_row.configuration_fingerprint IS NOT NULL)
            OR jsonb_array_length(g.roster)=0
            OR EXISTS (
                SELECT 1 FROM jsonb_array_elements(g.roster) resource
                WHERE NOT EXISTS (
                    SELECT 1 FROM collaboration_grant grant_row
                    WHERE grant_row.id=(resource->>'grant_id')::uuid
                        AND (grant_row.epoch=(resource->>'grant_epoch')::bigint OR
                            grant_row.revoked_at IS NOT NULL AND grant_row.epoch>(resource->>'grant_epoch')::bigint)
                        AND (resource->>'grant_epoch')::bigint>0
                        AND grant_row.target_id=g.target_id AND grant_row.target_epoch=g.target_epoch
                        AND grant_row.config_version=g.config_version
                        AND grant_row.configuration_fingerprint IS NOT NULL
                        AND grant_row.configuration_fingerprint=(SELECT original.configuration_fingerprint FROM collaboration_grant original
                            WHERE original.id=(g.roster->0->>'grant_id')::uuid)
                        AND grant_row.owner_participant_epoch IS NOT NULL
                        AND grant_row.agent_id::text=resource->>'agent_id'
                        AND grant_row.runtime_id::text=resource->>'runtime_id'
                        AND grant_row.execution_workspace_id::text=resource->>'execution_workspace_id'
                        AND grant_row.owner_user_id::text=resource->>'owner_user_id'
                        AND grant_row.role=resource->>'role'
                )
            )
        )
    ) THEN
        RAISE EXCEPTION 'retained collaboration dependencies cannot be reconstructed without their exact target consent';
    END IF;
END $$;

INSERT INTO collaboration_group_dependency(group_id,command_id,target_id,target_epoch,config_version,configuration_fingerprint,grants)
SELECT g.id,i.command_id,g.target_id,g.target_epoch,g.config_version,
    (SELECT grant_row.configuration_fingerprint FROM collaboration_grant grant_row
       WHERE grant_row.id=(g.roster->0->>'grant_id')::uuid),
    (SELECT jsonb_agg(jsonb_build_object(
        'role',grant_row.role,'agent_id',grant_row.agent_id::text,'runtime_id',grant_row.runtime_id::text,
        'execution_workspace_id',grant_row.execution_workspace_id::text,'owner_user_id',grant_row.owner_user_id::text,
        'grant_id',grant_row.id::text,'grant_epoch',resource->>'grant_epoch',
        'owner_auth_epoch',grant_row.owner_auth_epoch::text,'owner_participant_epoch',grant_row.owner_participant_epoch::text,
        'identity_kind',grant_row.identity_kind,'installation_id',grant_row.installation_id::text,
        'binding_id',grant_row.binding_id::text,'binding_epoch',grant_row.binding_epoch::text) ORDER BY grant_row.id)
     FROM jsonb_array_elements(g.roster) resource
     JOIN collaboration_grant grant_row ON grant_row.id=(resource->>'grant_id')::uuid
       AND grant_row.target_id=g.target_id AND grant_row.target_epoch=g.target_epoch
       AND grant_row.config_version=g.config_version)
FROM collaboration_group_input i JOIN collaboration_execution_group g ON g.id=i.group_id;

ALTER TABLE collaboration_run ADD COLUMN prompt_snapshot jsonb
    CHECK (prompt_snapshot IS NULL OR jsonb_typeof(prompt_snapshot)='array');

CREATE FUNCTION collaboration_preserve_group_dependency() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='DELETE' THEN
        IF EXISTS (SELECT 1 FROM collaboration_execution_group WHERE id=OLD.group_id) THEN
            RAISE EXCEPTION 'collaboration group dependencies retain their history' USING ERRCODE='23514';
        END IF;
        RETURN OLD;
    END IF;
    IF TG_OP='UPDATE' AND (
        ROW(NEW.group_id,NEW.command_id,NEW.target_id,NEW.target_epoch,NEW.config_version,NEW.configuration_fingerprint,NEW.grants)
        IS DISTINCT FROM ROW(OLD.group_id,OLD.command_id,OLD.target_id,OLD.target_epoch,OLD.config_version,OLD.configuration_fingerprint,OLD.grants)
        OR OLD.detached_at IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'collaboration dependency identity is immutable' USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER collaboration_group_dependency_identity BEFORE UPDATE OR DELETE ON collaboration_group_dependency
    FOR EACH ROW EXECUTE FUNCTION collaboration_preserve_group_dependency();

CREATE FUNCTION collaboration_preserve_run_prompt() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.prompt_snapshot IS NOT NULL AND NEW.prompt_snapshot IS DISTINCT FROM OLD.prompt_snapshot THEN
        RAISE EXCEPTION 'collaboration claimed prompt is immutable' USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER collaboration_run_prompt_identity BEFORE UPDATE ON collaboration_run
    FOR EACH ROW EXECUTE FUNCTION collaboration_preserve_run_prompt();
COMMIT;
