CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_collaboration_memory_version_run_request ON collaboration_memory_version(run_id, request_id) WHERE run_id IS NOT NULL;
