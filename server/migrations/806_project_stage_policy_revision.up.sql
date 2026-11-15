CREATE UNIQUE INDEX CONCURRENTLY project_stage_policy_revision_idx ON project_stage_policy (workspace_id, project_id, revision);
