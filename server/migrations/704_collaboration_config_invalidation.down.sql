DO $rollback$
BEGIN
    LOCK TABLE agent, agent_invocation_target, agent_runtime, agent_skill, collaboration_config_invalidation, squad, squad_member
        IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM collaboration_config_invalidation) THEN
        RAISE EXCEPTION 'cannot remove retained collaboration configuration invalidations';
    END IF;
    DROP TRIGGER IF EXISTS collaboration_agent_config_intent ON agent;
    DROP TRIGGER IF EXISTS collaboration_runtime_config_intent ON agent_runtime;
    DROP TRIGGER IF EXISTS collaboration_squad_config_intent ON squad;
    DROP TRIGGER IF EXISTS collaboration_invocation_config ON agent_invocation_target;
    DROP TRIGGER IF EXISTS collaboration_membership_config ON squad_member;
    DROP TRIGGER IF EXISTS collaboration_agent_skill_config ON agent_skill;
    DROP FUNCTION IF EXISTS collaboration_record_resource_config();
    DROP FUNCTION IF EXISTS collaboration_advance_child_resource_config();
    DROP TABLE collaboration_config_invalidation;

    EXECUTE $definition$
CREATE OR REPLACE FUNCTION collaboration_advance_resource_config() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE changed boolean;
BEGIN
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
        changed := ROW(NEW.workspace_id, NEW.owner_id, NEW.daemon_id, NEW.provider, NEW.runtime_mode, NEW.profile_id)
            IS DISTINCT FROM ROW(OLD.workspace_id, OLD.owner_id, OLD.daemon_id, OLD.provider, OLD.runtime_mode, OLD.profile_id);
    ELSIF TG_TABLE_NAME = 'squad' THEN
        changed := ROW(NEW.workspace_id, NEW.leader_id, NEW.archived_at, NEW.instructions)
            IS DISTINCT FROM ROW(OLD.workspace_id, OLD.leader_id, OLD.archived_at, OLD.instructions);
    ELSE
        RAISE EXCEPTION 'unsupported collaboration resource' USING ERRCODE = '23514';
    END IF;
    NEW.collaboration_config_epoch := OLD.collaboration_config_epoch + CASE WHEN changed THEN 1 ELSE 0 END;
    RETURN NEW;
END;
$$;
$definition$;
END;
$rollback$;
