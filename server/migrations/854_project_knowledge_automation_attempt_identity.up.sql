CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS collaboration_knowledge_attempt_identity_idx ON collaboration_knowledge_attempt (batch_id, execution_revision, stage, attempt_number);
