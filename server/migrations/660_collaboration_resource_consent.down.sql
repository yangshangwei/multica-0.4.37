DO $$
BEGIN
    LOCK TABLE agent, agent_runtime, collaboration_grant, collaboration_target, squad IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM collaboration_grant WHERE configuration_fingerprint IS NOT NULL)
        OR EXISTS (SELECT 1 FROM collaboration_target WHERE configuration_fingerprint IS NOT NULL) THEN
        RAISE EXCEPTION 'cannot remove retained collaboration resource consent';
    END IF;
    DROP TRIGGER collaboration_grant_consent ON collaboration_grant;
    DROP FUNCTION collaboration_preserve_resource_consent();
    ALTER TABLE collaboration_grant DROP CONSTRAINT collaboration_grant_consent_evidence;
    ALTER TABLE collaboration_grant DROP COLUMN owner_participant_epoch, DROP COLUMN target_epoch, DROP COLUMN configuration_fingerprint;
    ALTER TABLE collaboration_target DROP COLUMN configuration_fingerprint;
    DROP TRIGGER collaboration_agent_config ON agent;
    DROP TRIGGER collaboration_runtime_config ON agent_runtime;
    DROP TRIGGER collaboration_squad_config ON squad;
    DROP FUNCTION collaboration_advance_resource_config();
    ALTER TABLE agent DROP COLUMN collaboration_config_epoch;
    ALTER TABLE agent_runtime DROP COLUMN collaboration_config_epoch;
    ALTER TABLE squad DROP COLUMN collaboration_config_epoch;
END;
$$;
