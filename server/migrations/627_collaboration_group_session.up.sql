CREATE INDEX CONCURRENTLY idx_collaboration_group_session ON collaboration_execution_group (session_id, created_at, id);
