CREATE INDEX CONCURRENTLY IF NOT EXISTS collaboration_command_history_idx ON collaboration_command (session_id, created_at DESC, id DESC);
