CREATE UNIQUE INDEX CONCURRENTLY idx_collaboration_grant_target_resource ON collaboration_grant (target_id, config_version, role, agent_id, runtime_id) WHERE revoked_at IS NULL;
