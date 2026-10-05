CREATE UNIQUE INDEX CONCURRENTLY triage_intake_identity ON triage_intake_request (workspace_id, actor_id, request_id);
