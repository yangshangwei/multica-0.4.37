BEGIN;

CREATE TABLE IF NOT EXISTS collaboration_config_invalidation (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    resource_kind text NOT NULL CHECK (resource_kind IN ('agent', 'runtime', 'squad')),
    resource_id uuid NOT NULL,
    config_epoch bigint NOT NULL CHECK (config_epoch > 0),
    reason text NOT NULL CHECK (reason IN ('configuration_changed', 'resource_deleted')),
    last_issue_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_attempted_at timestamptz,
    reconciled_at timestamptz
);

-- Resource epochs deny old capabilities at the writer's commit. Child writers
-- may request one explicit advance under the same parent fence; neither API
-- payloads nor repeated no-op writes can assign a replacement epoch.
CREATE OR REPLACE FUNCTION collaboration_advance_resource_config() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE changed boolean;
BEGIN
    IF NEW.collaboration_config_epoch IS DISTINCT FROM OLD.collaboration_config_epoch
       AND NEW.collaboration_config_epoch <> OLD.collaboration_config_epoch + 1 THEN
        RAISE EXCEPTION 'invalid collaboration resource epoch advance' USING ERRCODE = '23514';
    END IF;
    IF TG_TABLE_NAME = 'agent' THEN
        changed := ROW(NEW.workspace_id, NEW.owner_id, NEW.runtime_id, NEW.kind, NEW.archived_at,
            NEW.permission_mode, NEW.runtime_mode, NEW.runtime_config, NEW.instructions, NEW.custom_env,
            NEW.custom_args, NEW.mcp_config, NEW.model, NEW.thinking_level, NEW.service_tier,
            NEW.composio_toolkit_allowlist, NEW.disabled_runtime_skills, NEW.autonomy_level)
            IS DISTINCT FROM
            ROW(OLD.workspace_id, OLD.owner_id, OLD.runtime_id, OLD.kind, OLD.archived_at,
            OLD.permission_mode, OLD.runtime_mode, OLD.runtime_config, OLD.instructions, OLD.custom_env,
            OLD.custom_args, OLD.mcp_config, OLD.model, OLD.thinking_level, OLD.service_tier,
            OLD.composio_toolkit_allowlist, OLD.disabled_runtime_skills, OLD.autonomy_level);
    ELSIF TG_TABLE_NAME = 'agent_runtime' THEN
        changed := ROW(NEW.workspace_id, NEW.owner_id, NEW.daemon_id, NEW.provider, NEW.runtime_mode, NEW.profile_id, NEW.visibility)
            IS DISTINCT FROM ROW(OLD.workspace_id, OLD.owner_id, OLD.daemon_id, OLD.provider, OLD.runtime_mode, OLD.profile_id, OLD.visibility);
    ELSIF TG_TABLE_NAME = 'squad' THEN
        changed := ROW(NEW.workspace_id, NEW.leader_id, NEW.archived_at, NEW.instructions)
            IS DISTINCT FROM ROW(OLD.workspace_id, OLD.leader_id, OLD.archived_at, OLD.instructions);
    ELSE
        RAISE EXCEPTION 'unsupported collaboration resource' USING ERRCODE = '23514';
    END IF;
    NEW.collaboration_config_epoch := OLD.collaboration_config_epoch +
        CASE WHEN changed OR NEW.collaboration_config_epoch <> OLD.collaboration_config_epoch THEN 1 ELSE 0 END;
    RETURN NEW;
END;
$$;

-- No source row or other resource is touched while the writer owns its resource
-- fence. The immutable outbox identity, not a process callback, survives commit.
CREATE OR REPLACE FUNCTION collaboration_record_resource_config() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE kind text;
BEGIN
    kind := CASE TG_TABLE_NAME WHEN 'agent_runtime' THEN 'runtime' ELSE TG_TABLE_NAME END;
    IF TG_OP = 'DELETE' THEN
        INSERT INTO collaboration_config_invalidation(resource_kind, resource_id, config_epoch, reason)
        VALUES (kind, OLD.id, OLD.collaboration_config_epoch + 1, 'resource_deleted');
    ELSIF NEW.collaboration_config_epoch IS DISTINCT FROM OLD.collaboration_config_epoch THEN
        INSERT INTO collaboration_config_invalidation(resource_kind, resource_id, config_epoch, reason)
        VALUES (kind, NEW.id, NEW.collaboration_config_epoch, 'configuration_changed');
    END IF;
    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS collaboration_agent_config_intent ON agent;
CREATE TRIGGER collaboration_agent_config_intent AFTER UPDATE OR DELETE ON agent
    FOR EACH ROW EXECUTE FUNCTION collaboration_record_resource_config();
DROP TRIGGER IF EXISTS collaboration_runtime_config_intent ON agent_runtime;
CREATE TRIGGER collaboration_runtime_config_intent AFTER UPDATE OR DELETE ON agent_runtime
    FOR EACH ROW EXECUTE FUNCTION collaboration_record_resource_config();
DROP TRIGGER IF EXISTS collaboration_squad_config_intent ON squad;
CREATE TRIGGER collaboration_squad_config_intent AFTER UPDATE OR DELETE ON squad
    FOR EACH ROW EXECUTE FUNCTION collaboration_record_resource_config();

-- Existing writers lock parents before children. A reverse bulk/cleanup writer
-- cannot wait on a parent while owning child rows: its complete transaction must
-- roll back on NOWAIT. New and old parent identities are sorted for moves.
CREATE OR REPLACE FUNCTION collaboration_advance_child_resource_config() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE parents uuid[] := '{}'; parent uuid;
BEGIN
    IF TG_OP = 'UPDATE' THEN
        IF TG_TABLE_NAME = 'agent_invocation_target' THEN
            -- The frozen fingerprint retains invocation identity/provenance.
            IF NEW IS NOT DISTINCT FROM OLD THEN RETURN NULL; END IF;
        ELSIF TG_TABLE_NAME = 'squad_member' THEN
            IF ROW(NEW.id, NEW.squad_id, NEW.member_type, NEW.member_id, NEW.role) IS NOT DISTINCT FROM ROW(OLD.id, OLD.squad_id, OLD.member_type, OLD.member_id, OLD.role) THEN RETURN NULL; END IF;
        ELSIF TG_TABLE_NAME = 'agent_skill' THEN
            IF ROW(NEW.agent_id, NEW.skill_id, NEW.enabled) IS NOT DISTINCT FROM ROW(OLD.agent_id, OLD.skill_id, OLD.enabled) THEN RETURN NULL; END IF;
        END IF;
    END IF;
    IF TG_TABLE_NAME IN ('agent_invocation_target', 'agent_skill') THEN
        IF TG_OP <> 'INSERT' THEN parents := array_append(parents, OLD.agent_id); END IF;
        IF TG_OP <> 'DELETE' THEN parents := array_append(parents, NEW.agent_id); END IF;
    ELSIF TG_TABLE_NAME = 'squad_member' THEN
        IF TG_OP <> 'INSERT' AND OLD.member_type = 'agent' THEN parents := array_append(parents, OLD.squad_id); END IF;
        IF TG_OP <> 'DELETE' AND NEW.member_type = 'agent' THEN parents := array_append(parents, NEW.squad_id); END IF;
    ELSE
        RAISE EXCEPTION 'unsupported collaboration configuration child' USING ERRCODE = '23514';
    END IF;
    FOR parent IN SELECT DISTINCT id FROM unnest(parents) AS changed(id) WHERE id IS NOT NULL ORDER BY id LOOP
        IF TG_TABLE_NAME IN ('agent_invocation_target', 'agent_skill') THEN
            PERFORM id FROM agent WHERE id = parent FOR UPDATE NOWAIT;
            UPDATE agent SET collaboration_config_epoch = collaboration_config_epoch + 1 WHERE id = parent;
        ELSE
            PERFORM id FROM squad WHERE id = parent FOR UPDATE NOWAIT;
            UPDATE squad SET collaboration_config_epoch = collaboration_config_epoch + 1 WHERE id = parent;
        END IF;
    END LOOP;
    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS collaboration_invocation_config ON agent_invocation_target;
CREATE TRIGGER collaboration_invocation_config AFTER INSERT OR UPDATE OR DELETE ON agent_invocation_target
    FOR EACH ROW EXECUTE FUNCTION collaboration_advance_child_resource_config();
DROP TRIGGER IF EXISTS collaboration_membership_config ON squad_member;
CREATE TRIGGER collaboration_membership_config AFTER INSERT OR UPDATE OR DELETE ON squad_member
    FOR EACH ROW EXECUTE FUNCTION collaboration_advance_child_resource_config();
DROP TRIGGER IF EXISTS collaboration_agent_skill_config ON agent_skill;
CREATE TRIGGER collaboration_agent_skill_config AFTER INSERT OR UPDATE OR DELETE ON agent_skill
    FOR EACH ROW EXECUTE FUNCTION collaboration_advance_child_resource_config();

-- Sweep previously changed or missing resources too. Current fingerprint
-- validation preserves valid configurations and never revives obsolete grants.
-- Re-running this migration leaves its original durable intent identities alone.
WITH retained AS (
    SELECT 'squad'::text AS kind, t.resource_id AS id FROM collaboration_target t WHERE t.kind = 'squad'
    UNION
    SELECT 'agent', (resource->>'agent_id')::uuid FROM collaboration_target t
    CROSS JOIN LATERAL jsonb_array_elements(t.roster) resource
    UNION
    SELECT 'runtime', (resource->>'runtime_id')::uuid FROM collaboration_target t
    CROSS JOIN LATERAL jsonb_array_elements(t.roster) resource
    UNION
    SELECT 'agent', agent_id FROM collaboration_grant
    UNION
    SELECT 'runtime', runtime_id FROM collaboration_grant
    UNION
    SELECT 'agent', (resource->>'agent_id')::uuid FROM collaboration_execution_group g
    CROSS JOIN LATERAL jsonb_array_elements(g.roster) resource WHERE g.settled_at IS NULL
    UNION
    SELECT 'runtime', (resource->>'runtime_id')::uuid FROM collaboration_execution_group g
    CROSS JOIN LATERAL jsonb_array_elements(g.roster) resource WHERE g.settled_at IS NULL
    UNION
    SELECT 'agent', agent_id FROM collaboration_run WHERE settled_at IS NULL
    UNION
    SELECT 'runtime', runtime_id FROM collaboration_run WHERE settled_at IS NULL
)
INSERT INTO collaboration_config_invalidation(resource_kind, resource_id, config_epoch, reason)
SELECT retained.kind, retained.id, COALESCE(a.collaboration_config_epoch, r.collaboration_config_epoch, s.collaboration_config_epoch, 1),
    CASE WHEN COALESCE(a.id, r.id, s.id) IS NULL THEN 'resource_deleted' ELSE 'configuration_changed' END
FROM retained
LEFT JOIN agent a ON retained.kind = 'agent' AND a.id = retained.id
LEFT JOIN agent_runtime r ON retained.kind = 'runtime' AND r.id = retained.id
LEFT JOIN squad s ON retained.kind = 'squad' AND s.id = retained.id
WHERE retained.id IS NOT NULL AND NOT EXISTS (
    SELECT 1 FROM collaboration_config_invalidation intent
    WHERE intent.resource_kind = retained.kind AND intent.resource_id = retained.id
);

COMMIT;
