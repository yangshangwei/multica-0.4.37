CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS collaboration_knowledge_scan_cause_idx ON collaboration_knowledge_invalidation (cause_id) WHERE kind = 'scan';
