CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS project_update_request_identity ON project_update_request (workspace_id, actor_user_id, request_id);
