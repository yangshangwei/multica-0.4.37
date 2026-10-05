CREATE INDEX CONCURRENTLY IF NOT EXISTS project_update_timeline ON project_update (workspace_id, project_id, published_at DESC, id DESC);
