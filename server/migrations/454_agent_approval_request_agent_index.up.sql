CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_agent_approval_request_agent
    ON agent_approval_request (agent_id, created_at DESC);
