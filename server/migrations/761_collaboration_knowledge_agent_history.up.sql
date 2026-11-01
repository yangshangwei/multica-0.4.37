CREATE INDEX CONCURRENTLY IF NOT EXISTS collaboration_knowledge_agent_history_idx
ON agent_task_queue (agent_id, created_at DESC, id DESC)
WHERE knowledge_context_version IS NOT NULL;
