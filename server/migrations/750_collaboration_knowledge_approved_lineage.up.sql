CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS collaboration_knowledge_approved_lineage_idx ON collaboration_knowledge_candidate (lineage_id) WHERE state = 'approved';
