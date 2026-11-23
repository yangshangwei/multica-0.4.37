CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_collaboration_group_input_identity ON collaboration_group_input(group_id,command_id);
