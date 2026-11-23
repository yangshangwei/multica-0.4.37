CREATE INDEX CONCURRENTLY IF NOT EXISTS collaboration_knowledge_evaluation_candidate_idx ON collaboration_knowledge_evaluation (candidate_id, created_at, id);
