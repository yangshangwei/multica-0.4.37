CREATE INDEX CONCURRENTLY idx_collaboration_run_parent ON collaboration_run (parent_run_id) WHERE parent_run_id IS NOT NULL;
