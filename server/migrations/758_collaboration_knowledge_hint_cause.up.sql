CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS collaboration_knowledge_hint_cause_idx ON collaboration_knowledge_invalidation (cause_id, target_kind, target_id) WHERE kind = 'target_hint';
