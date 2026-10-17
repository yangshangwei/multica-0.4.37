CREATE UNIQUE INDEX CONCURRENTLY idx_collaboration_delegation_request ON collaboration_delegation (parent_run_id, request_key);
