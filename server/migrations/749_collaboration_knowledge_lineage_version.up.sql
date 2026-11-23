CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS collaboration_knowledge_lineage_version_idx ON collaboration_knowledge_candidate (lineage_id, version);
