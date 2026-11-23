CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS collaboration_knowledge_batch_slot_idx ON collaboration_knowledge_batch (job_id, batch_index);
