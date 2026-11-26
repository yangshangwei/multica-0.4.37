CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS git_material_request_identity ON git_material_request (workspace_id, project_id, actor_id, request_id);
