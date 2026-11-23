BEGIN;

-- Restores the migration 704 epoch writer before dropping the boundary columns.
-- Targets lose owner re-consent recovery and return to Switch-only.
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

ALTER TABLE collaboration_target DROP COLUMN IF EXISTS configuration_boundary_fingerprint;
ALTER TABLE agent DROP COLUMN IF EXISTS collaboration_boundary_epoch;

COMMIT;
