CREATE UNIQUE INDEX CONCURRENTLY idx_collaboration_run_group_step ON collaboration_run (group_id, step_key);
