CREATE UNIQUE INDEX CONCURRENTLY idx_collaboration_group_batch ON collaboration_execution_group(batch_id) WHERE batch_id IS NOT NULL;
