CREATE UNIQUE INDEX CONCURRENTLY triage_action_request ON triage_action (workspace_id, actor_id, request_id);
