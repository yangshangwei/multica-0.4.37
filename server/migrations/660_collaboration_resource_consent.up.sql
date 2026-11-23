BEGIN;

-- These versions describe execution authority/configuration, never presence.
-- Row-local triggers cover every writer without creating cross-table effects.
ALTER TABLE agent ADD COLUMN collaboration_config_epoch bigint NOT NULL DEFAULT 1 CHECK (collaboration_config_epoch > 0);
ALTER TABLE agent_runtime ADD COLUMN collaboration_config_epoch bigint NOT NULL DEFAULT 1 CHECK (collaboration_config_epoch > 0);
ALTER TABLE squad ADD COLUMN collaboration_config_epoch bigint NOT NULL DEFAULT 1 CHECK (collaboration_config_epoch > 0);

CREATE FUNCTION collaboration_advance_resource_config() RETURNS trigger LANGUAGE plpgsql AS $$
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

CREATE TRIGGER collaboration_agent_config BEFORE UPDATE ON agent FOR EACH ROW EXECUTE FUNCTION collaboration_advance_resource_config();
CREATE TRIGGER collaboration_runtime_config BEFORE UPDATE ON agent_runtime FOR EACH ROW EXECUTE FUNCTION collaboration_advance_resource_config();
CREATE TRIGGER collaboration_squad_config BEFORE UPDATE ON squad FOR EACH ROW EXECUTE FUNCTION collaboration_advance_resource_config();

-- Existing foundation grants deliberately have no consent evidence. They must
-- not become authorized simply because their resource IDs still happen to exist.
ALTER TABLE collaboration_target ADD COLUMN configuration_fingerprint text CHECK (configuration_fingerprint ~ '^[0-9a-f]{64}$');
ALTER TABLE collaboration_grant ADD COLUMN owner_participant_epoch bigint CHECK (owner_participant_epoch > 0);
ALTER TABLE collaboration_grant ADD COLUMN target_epoch bigint CHECK (target_epoch > 0);
ALTER TABLE collaboration_grant ADD COLUMN configuration_fingerprint text CHECK (configuration_fingerprint ~ '^[0-9a-f]{64}$');
ALTER TABLE collaboration_grant ADD CONSTRAINT collaboration_grant_consent_evidence CHECK (
    (owner_participant_epoch IS NULL AND target_epoch IS NULL AND configuration_fingerprint IS NULL) OR
    (owner_participant_epoch IS NOT NULL AND target_epoch IS NOT NULL AND configuration_fingerprint IS NOT NULL)
);

CREATE FUNCTION collaboration_preserve_resource_consent() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF ROW(NEW.owner_participant_epoch, NEW.target_epoch, NEW.configuration_fingerprint)
        IS DISTINCT FROM ROW(OLD.owner_participant_epoch, OLD.target_epoch, OLD.configuration_fingerprint) THEN
        RAISE EXCEPTION 'collaboration consent identity is immutable' USING ERRCODE = '23514';
    END IF;
    IF OLD.revoked_at IS NOT NULL AND NEW.revoked_at IS DISTINCT FROM OLD.revoked_at THEN
        RAISE EXCEPTION 'collaboration resource consent cannot be revived' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER collaboration_grant_consent BEFORE UPDATE ON collaboration_grant FOR EACH ROW EXECUTE FUNCTION collaboration_preserve_resource_consent();

COMMIT;
