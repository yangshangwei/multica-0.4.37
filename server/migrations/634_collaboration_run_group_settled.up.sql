CREATE INDEX CONCURRENTLY idx_collaboration_run_group_settled ON collaboration_run (group_id, settled_at);
