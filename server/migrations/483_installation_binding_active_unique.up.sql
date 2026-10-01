CREATE UNIQUE INDEX CONCURRENTLY installation_binding_active_uidx ON installation_daemon_binding (workspace_id, daemon_id) WHERE state = 'active';
