CREATE INDEX CONCURRENTLY IF NOT EXISTS collaboration_knowledge_runtime_history_idx
ON agent_task_queue (runtime_id, created_at DESC, id DESC)
WHERE knowledge_context_version IS NOT NULL;
