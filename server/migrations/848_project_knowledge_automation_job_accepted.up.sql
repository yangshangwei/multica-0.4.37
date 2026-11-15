CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS collaboration_knowledge_job_accepted_idx ON collaboration_knowledge_job (source_issue_id, accepted_event_seq);
