CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS project_state_change_revision ON project_state_change (workspace_id, project_id, project_revision);
