CREATE UNIQUE INDEX CONCURRENTLY project_stage_policy_request_idx ON project_stage_policy (workspace_id, project_id, created_by, request_id);
