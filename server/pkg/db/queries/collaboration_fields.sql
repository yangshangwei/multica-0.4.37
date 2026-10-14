-- These catalogs are internal inputs to an object-authorized projection. Never
-- substitute the ordinary catalog endpoints (which include usage/management data).
-- name: ListCollaborationFieldLabels :many
SELECT id, name, color FROM issue_label
WHERE workspace_id = sqlc.arg(workspace_id)::uuid AND resource_type = 'issue'
ORDER BY lower(name), id;

-- name: ListCollaborationFieldProperties :many
SELECT * FROM issue_property
WHERE workspace_id = sqlc.arg(workspace_id)::uuid
  AND (sqlc.arg(include_archived)::boolean OR archived_at IS NULL)
ORDER BY position, lower(name), id;

-- The workflow already holds source issue/session locks. These post-source
-- locks must never wait on an ordinary property writer or member teardown.
-- name: TryLockCollaborationFieldProperty :one
SELECT pg_try_advisory_xact_lock(hashtextextended('prop:' || sqlc.arg(property_id)::uuid::text, 0))::boolean;

-- name: LockCollaborationFieldProperties :many
SELECT * FROM issue_property
WHERE workspace_id = sqlc.arg(workspace_id)::uuid
  AND id = ANY(sqlc.arg(property_ids)::uuid[])
ORDER BY id FOR SHARE NOWAIT;

-- name: LockCollaborationFieldLabels :many
SELECT * FROM issue_label
WHERE workspace_id = sqlc.arg(workspace_id)::uuid AND resource_type = 'issue'
  AND id = ANY(sqlc.arg(label_ids)::uuid[])
ORDER BY id FOR SHARE NOWAIT;

-- Actor values are passive references to source members, identified by user.id.
-- Holding these rows protects validation against a concurrent membership removal.
-- name: LockCollaborationFieldMembers :many
SELECT user_id FROM member
WHERE workspace_id = sqlc.arg(workspace_id)::uuid
  AND user_id = ANY(sqlc.arg(user_ids)::uuid[])
ORDER BY user_id FOR SHARE NOWAIT;

-- Same whole-bag limit as issue_properties_size; validate before required writes
-- so a refusal can be saved with the original operation key.
-- name: CheckCollaborationProperties :one
SELECT (pg_column_size(sqlc.arg(properties)::jsonb) <= 16384)::boolean AS fits,
       (sqlc.arg(properties)::jsonb = sqlc.arg(current_properties)::jsonb)::boolean AS unchanged;
