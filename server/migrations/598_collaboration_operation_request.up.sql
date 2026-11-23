CREATE UNIQUE INDEX CONCURRENTLY collaboration_operation_request_idx ON collaboration_operation (scope_kind, scope_id, actor_user_id, request_id);
