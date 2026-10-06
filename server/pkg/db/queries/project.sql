-- name: ListProjects :many
SELECT * FROM project
WHERE workspace_id = $1
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('priority')::text IS NULL OR priority = sqlc.narg('priority'))
ORDER BY created_at DESC;

-- name: GetProjectInWorkspace :one
SELECT * FROM project
WHERE id = $1 AND workspace_id = $2;

-- name: LockProjectForExecutionSquad :one
-- Shares the exclusive project lock with deletion. The selection revision is
-- compared under this lock before any template resources are materialized.
SELECT * FROM project
WHERE id = $1 AND workspace_id = $2
FOR UPDATE;

-- name: UpdateProjectExecutionSquad :one
UPDATE project SET revision = revision + CASE WHEN execution_squad IS DISTINCT FROM $3::jsonb THEN 1 ELSE 0 END, execution_squad = $3, updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: LockProjectForChatSessionCreate :one
-- Conflicts with project deletion so a chat session cannot commit a soft
-- project reference after the delete transaction has swept existing sessions.
SELECT id FROM project
WHERE id = $1 AND workspace_id = $2
FOR KEY SHARE;

-- name: LockProjectForDelete :one
-- Serializes project deletion with chat-session creation. The handler locks,
-- clears every soft chat reference, and deletes the project in one transaction.
SELECT id FROM project
WHERE id = $1 AND workspace_id = $2
FOR UPDATE;

-- name: CreateProject :one
INSERT INTO project (
    workspace_id, title, description, icon, status,
    lead_type, lead_id, priority, start_date, due_date, execution_squad, in_progress_since, in_progress_since_source
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, COALESCE(sqlc.narg('execution_squad')::jsonb, '{}'::jsonb), CASE WHEN $5::text = 'in_progress' THEN now() END, CASE WHEN $5::text = 'in_progress' THEN 'transition' END
) RETURNING *;

-- name: UpdateProject :one
UPDATE project SET
    title = COALESCE(sqlc.narg('title'), title),
    description = sqlc.narg('description'),
    icon = sqlc.narg('icon'),
    status = COALESCE(sqlc.narg('status'), status),
    priority = COALESCE(sqlc.narg('priority'), priority),
    lead_type = sqlc.narg('lead_type'),
    lead_id = sqlc.narg('lead_id'),
    start_date = sqlc.narg('start_date'),
    due_date = sqlc.narg('due_date'),
    description_revision = description_revision + CASE WHEN description IS DISTINCT FROM sqlc.narg('description')::text THEN 1 ELSE 0 END,
    revision = revision + 1,
    in_progress_since = CASE WHEN COALESCE(sqlc.narg('status'), status) <> 'in_progress' THEN NULL WHEN status <> 'in_progress' THEN now() ELSE in_progress_since END,
    in_progress_since_source = CASE WHEN COALESCE(sqlc.narg('status'), status) <> 'in_progress' THEN NULL WHEN status <> 'in_progress' THEN 'transition' ELSE in_progress_since_source END,
    updated_at = now()
WHERE id = $1 AND workspace_id = sqlc.arg('workspace_id')
RETURNING *;

-- name: DeleteProject :exec
-- Defense-in-depth: workspace_id is a SQL-layer tenant guard. See DeleteIssue.
DELETE FROM project WHERE id = $1 AND workspace_id = $2;

-- name: CountIssuesByProject :one
SELECT count(*) FROM issue
WHERE project_id = $1 AND admission_status IN ('not_required', 'accepted');

-- name: GetProjectIssueStats :many
SELECT project_id,
       count(*)::bigint AS total_count,
       count(*) FILTER (WHERE status = ANY(sqlc.arg('terminal_status_keys')::text[]))::bigint AS done_count
FROM issue
WHERE admission_status IN ('not_required', 'accepted') AND workspace_id = sqlc.arg('workspace_id')::uuid
  AND project_id = ANY(sqlc.arg('project_ids')::uuid[])
GROUP BY project_id;

-- name: LockProjectForAssociation :one
SELECT * FROM project WHERE id = $1 AND workspace_id = $2 FOR SHARE;

-- name: LockProjectForAssociationNowait :one
SELECT * FROM project WHERE id = $1 AND workspace_id = $2 FOR SHARE NOWAIT;

-- name: CreateProjectStateChange :exec
INSERT INTO project_state_change (workspace_id, project_id, actor_type, actor_id, from_status, to_status, reason, project_revision)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8);

-- name: GetProjectDeleteImpact :one
SELECT p.revision AS project_revision,
 (SELECT count(*) FROM issue WHERE workspace_id=p.workspace_id AND project_id=p.id)::bigint AS issue_count,
 (SELECT count(*) FROM issue WHERE workspace_id=p.workspace_id AND project_id=p.id AND admission_status IN ('not_required','accepted'))::bigint AS formal_issue_count,
 (SELECT count(*) FROM project_resource WHERE workspace_id=p.workspace_id AND project_id=p.id)::bigint AS resource_count,
 (SELECT count(*) FROM project_update WHERE workspace_id=p.workspace_id AND project_id=p.id)::bigint AS update_count,
 (SELECT count(*) FROM autopilot WHERE workspace_id=p.workspace_id AND project_id=p.id)::bigint AS autopilot_count
FROM project p WHERE p.id=$1 AND p.workspace_id=$2;

-- name: LockProjectAutopilotsForDelete :many
SELECT id FROM autopilot WHERE project_id=$1 AND workspace_id=$2 ORDER BY id FOR UPDATE;

-- name: DisableProjectAutopilotTriggers :exec
UPDATE autopilot_trigger SET enabled=false, updated_at=now()
WHERE autopilot_id IN (SELECT id FROM autopilot WHERE project_id=$1 AND workspace_id=$2);

-- name: DetachProjectAutopilots :exec
UPDATE autopilot SET project_id=NULL,
 status=CASE WHEN status='archived' THEN 'archived' ELSE 'paused' END,
 pause_reason=CASE WHEN status='archived' THEN pause_reason ELSE 'project_deleted' END,
 updated_at=now()
WHERE project_id=$1 AND workspace_id=$2;

-- name: DetachProjectIssues :exec
UPDATE issue SET project_id=NULL, revision=revision+1, updated_at=now()
WHERE project_id=$1 AND workspace_id=$2;

-- name: DeleteProjectResources :exec
DELETE FROM project_resource WHERE project_id=$1 AND workspace_id=$2;

-- name: DeleteProjectProgressInbox :exec
DELETE FROM inbox_item i WHERE i.workspace_id=$2
AND i.id IN (SELECT n.id FROM project_update_notification n WHERE n.project_id=$1 AND n.workspace_id=$2);

-- name: DeleteProjectProgress :exec
WITH notifications AS (DELETE FROM project_update_notification WHERE project_update_notification.project_id=$1 AND project_update_notification.workspace_id=$2),
 requests AS (DELETE FROM project_update_request WHERE project_update_request.project_id=$1 AND project_update_request.workspace_id=$2),
 revisions AS (DELETE FROM project_update_revision WHERE project_update_revision.project_id=$1 AND project_update_revision.workspace_id=$2),
 states AS (DELETE FROM project_state_change WHERE project_state_change.project_id=$1 AND project_state_change.workspace_id=$2)
DELETE FROM project_update WHERE project_update.project_id=$1 AND project_update.workspace_id=$2;
-- name: DeleteProjectIssueViewPreferences :exec
DELETE FROM issue_view_preference
WHERE workspace_id = sqlc.arg('workspace_id')
  AND scope_type = 'project'
  AND scope_id = sqlc.arg('scope_id');

-- name: ListProjectIssuesForDelete :many
-- Caller owns the exclusive project lock and iteration workspace fence.
SELECT * FROM issue WHERE project_id=$1 AND workspace_id=$2 ORDER BY id;

-- name: LockProjectIssuesForDelete :many
SELECT * FROM issue WHERE project_id=$1 AND workspace_id=$2 ORDER BY id FOR UPDATE;

-- name: LockProjectAutopilotTriggersForDelete :many
SELECT t.id FROM autopilot_trigger t
JOIN autopilot a ON a.id=t.autopilot_id
WHERE a.project_id=$1 AND a.workspace_id=$2 ORDER BY t.id FOR UPDATE OF t;

-- name: ProjectHasForeignReferences :one
-- Existing project FK children use SET NULL (issue/autopilot) or CASCADE
-- (project_resource). Reject malformed cross-tenant references before deletion
-- can invoke those legacy effects outside the tenant-scoped cleanup statements.
SELECT EXISTS(
 SELECT 1 FROM issue i WHERE i.project_id=sqlc.arg('project_id') AND i.workspace_id<>sqlc.arg('workspace_id')
 UNION ALL
 SELECT 1 FROM autopilot a WHERE a.project_id=sqlc.arg('project_id') AND a.workspace_id<>sqlc.arg('workspace_id')
 UNION ALL
 SELECT 1 FROM project_resource r WHERE r.project_id=sqlc.arg('project_id') AND r.workspace_id<>sqlc.arg('workspace_id')
);
