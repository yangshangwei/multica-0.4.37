CREATE UNIQUE INDEX CONCURRENTLY delivery_completion_request_idx ON delivery_completion_authorization (workspace_id, issuer_user_id, request_key);
