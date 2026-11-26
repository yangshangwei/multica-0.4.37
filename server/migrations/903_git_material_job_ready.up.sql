CREATE INDEX CONCURRENTLY IF NOT EXISTS git_material_job_ready ON git_material_job (next_at, leased_until);
