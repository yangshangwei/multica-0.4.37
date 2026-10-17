CREATE UNIQUE INDEX CONCURRENTLY idx_collaboration_target_identity ON collaboration_target (session_id, kind, resource_id);
