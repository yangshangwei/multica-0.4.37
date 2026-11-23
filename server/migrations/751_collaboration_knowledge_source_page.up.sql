CREATE INDEX CONCURRENTLY IF NOT EXISTS collaboration_knowledge_source_page_idx ON collaboration_knowledge_candidate (source_issue_id, proposed_at, id);
