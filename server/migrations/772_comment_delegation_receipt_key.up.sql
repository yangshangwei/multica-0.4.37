CREATE UNIQUE INDEX CONCURRENTLY comment_delegation_receipt_key_idx ON comment_delegation_receipt (workspace_id, actor_type, actor_id, originator_id, request_key);
