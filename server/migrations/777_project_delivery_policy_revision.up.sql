CREATE UNIQUE INDEX CONCURRENTLY project_delivery_policy_revision_idx ON project_delivery_policy (workspace_id, project_id, revision);
