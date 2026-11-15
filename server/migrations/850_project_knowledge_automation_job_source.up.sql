CREATE INDEX CONCURRENTLY IF NOT EXISTS collaboration_knowledge_job_source_idx ON collaboration_knowledge_job (source_workspace_id, source_issue_id, accepted_event_seq DESC);
