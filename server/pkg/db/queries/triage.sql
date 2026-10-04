-- name: GetTriageSettings :one
SELECT * FROM workspace_triage_settings WHERE workspace_id = $1;
-- name: GetIssueTriage :one
SELECT * FROM issue_triage WHERE issue_id = $1 AND workspace_id = $2;
-- name: GetTriageAction :one
SELECT * FROM triage_action WHERE id = $1 AND workspace_id = $2;
-- name: ListTriageIssueActions :many
SELECT * FROM triage_action WHERE issue_id = $1 AND workspace_id = $2 ORDER BY created_at, id;
-- name: GetTriageImportBatch :one
SELECT * FROM triage_import_batch WHERE id = $1 AND workspace_id = $2;
-- name: ListTriageImportRows :many
SELECT * FROM triage_import_row WHERE batch_id = $1 AND workspace_id = $2 ORDER BY row_number;
