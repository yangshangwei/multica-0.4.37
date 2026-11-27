CREATE UNIQUE INDEX CONCURRENTLY material_repository_job_idx ON git_material_job (repository_revision_id,kind);
