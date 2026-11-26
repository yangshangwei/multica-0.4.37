CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS git_material_job_revision ON git_material_job (revision_id, kind);
