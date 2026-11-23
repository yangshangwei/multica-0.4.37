CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_collaboration_memory_version_issue ON collaboration_memory_version(issue_id, version);
