CREATE UNIQUE INDEX CONCURRENTLY idx_collaboration_group_unsettled ON collaboration_execution_group (target_id) WHERE settled_at IS NULL;
