CREATE INDEX CONCURRENTLY IF NOT EXISTS collaboration_knowledge_delivery_history_idx
ON agent_task_queue USING gin (knowledge_delivery jsonb_path_ops)
WHERE knowledge_context_version IS NOT NULL;
