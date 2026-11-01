CREATE INDEX CONCURRENTLY IF NOT EXISTS collaboration_knowledge_source_workspace_idx ON collaboration_knowledge_candidate (source_workspace_id, id) WHERE published_target_revision IS NOT NULL;
