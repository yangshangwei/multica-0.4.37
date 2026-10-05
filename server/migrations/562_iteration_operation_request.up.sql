CREATE UNIQUE INDEX CONCURRENTLY iteration_operation_request ON iteration_operation (workspace_id, actor_user_id, request_id);
