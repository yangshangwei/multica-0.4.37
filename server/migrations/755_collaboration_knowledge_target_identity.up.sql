CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS collaboration_knowledge_target_identity_idx ON collaboration_knowledge_target (target_kind, target_id);
