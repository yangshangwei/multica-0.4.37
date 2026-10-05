CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS project_update_revision_version ON project_update_revision (update_id, revision);
