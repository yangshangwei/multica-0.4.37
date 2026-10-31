CREATE INDEX CONCURRENTLY IF NOT EXISTS collaboration_knowledge_source_issue_idx ON collaboration_knowledge_candidate (source_issue_id, id) WHERE published_target_revision IS NOT NULL;
