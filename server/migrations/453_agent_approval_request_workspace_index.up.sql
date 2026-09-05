CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_agent_approval_request_workspace
    ON agent_approval_request (workspace_id, status, created_at DESC);
