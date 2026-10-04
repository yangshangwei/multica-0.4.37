CREATE INDEX CONCURRENTLY triage_import_external ON triage_import_row (workspace_id, external_id) WHERE issue_id IS NOT NULL;
