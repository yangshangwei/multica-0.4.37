BEGIN;

-- The boundary epoch counts agent changes that may widen execution mode or
-- managed tool permissions. Instructions, model and thinking level advance only
-- the configuration epoch, so the resource owner may re-confirm them directly.
-- NULL counts as zero; only this trigger advances it.
ALTER TABLE agent ADD COLUMN IF NOT EXISTS collaboration_boundary_epoch bigint
    CHECK (collaboration_boundary_epoch IS NULL OR collaboration_boundary_epoch > 0);

-- The digest of the target configuration with each agent's boundary epoch in
-- place of its configuration epoch. NULL keeps a target Switch-only, including
-- every target registered before this column existed.
ALTER TABLE collaboration_target ADD COLUMN IF NOT EXISTS configuration_boundary_fingerprint text
    CHECK (configuration_boundary_fingerprint IS NULL OR configuration_boundary_fingerprint ~ '^[0-9a-f]{64}$');

-- Same contract as migration 704, plus the agent boundary epoch. A child writer
-- that requests one explicit configuration advance (invocation targets, skill
-- bindings, skill content, plugin releases) also widens the boundary, because
-- those writers cannot say which part of the agent's tools changed.
CREATE OR REPLACE FUNCTION collaboration_advance_resource_config() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE changed boolean; widened boolean := false; requested boolean;
BEGIN
    IF NEW.collaboration_config_epoch IS DISTINCT FROM OLD.collaboration_config_epoch
       AND NEW.collaboration_config_epoch <> OLD.collaboration_config_epoch + 1 THEN
        RAISE EXCEPTION 'invalid collaboration resource epoch advance' USING ERRCODE = '23514';
    END IF;
    requested := NEW.collaboration_config_epoch <> OLD.collaboration_config_epoch;
    IF TG_TABLE_NAME = 'agent' THEN
        IF NEW.collaboration_boundary_epoch IS DISTINCT FROM OLD.collaboration_boundary_epoch THEN
            RAISE EXCEPTION 'invalid collaboration boundary epoch advance' USING ERRCODE = '23514';
        END IF;
        widened := ROW(NEW.workspace_id, NEW.owner_id, NEW.runtime_id, NEW.kind, NEW.archived_at,
            NEW.permission_mode, NEW.runtime_mode, NEW.runtime_config, NEW.custom_env,
            NEW.custom_args, NEW.mcp_config, NEW.service_tier,
            NEW.composio_toolkit_allowlist, NEW.disabled_runtime_skills, NEW.autonomy_level)
            IS DISTINCT FROM
            ROW(OLD.workspace_id, OLD.owner_id, OLD.runtime_id, OLD.kind, OLD.archived_at,
            OLD.permission_mode, OLD.runtime_mode, OLD.runtime_config, OLD.custom_env,
            OLD.custom_args, OLD.mcp_config, OLD.service_tier,
            OLD.composio_toolkit_allowlist, OLD.disabled_runtime_skills, OLD.autonomy_level);
        changed := widened OR ROW(NEW.instructions, NEW.model, NEW.thinking_level)
            IS DISTINCT FROM ROW(OLD.instructions, OLD.model, OLD.thinking_level);
        IF widened OR requested THEN
            NEW.collaboration_boundary_epoch := COALESCE(OLD.collaboration_boundary_epoch, 0) + 1;
        END IF;
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
        CASE WHEN changed OR requested THEN 1 ELSE 0 END;
    RETURN NEW;
END;
$$;

COMMIT;
