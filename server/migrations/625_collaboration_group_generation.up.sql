CREATE UNIQUE INDEX CONCURRENTLY idx_collaboration_group_generation ON collaboration_execution_group (target_id, generation);
