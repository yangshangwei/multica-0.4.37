-- Shared context reads run inside the caller's issue-authorized transaction.
-- A missing row set is version 0, the empty document.

-- name: GetCollaborationMemoryMaxVersion :one
SELECT COALESCE(max(version), 0)::bigint AS version
FROM collaboration_memory_version WHERE issue_id = sqlc.arg(issue_id)::uuid;

-- name: GetCollaborationMemoryCurrent :one
SELECT * FROM collaboration_memory_version
WHERE issue_id = sqlc.arg(issue_id)::uuid
ORDER BY version DESC LIMIT 1;

-- name: GetCollaborationMemoryVersion :one
SELECT * FROM collaboration_memory_version
WHERE issue_id = sqlc.arg(issue_id)::uuid AND version = sqlc.arg(version)::bigint;

-- name: LockCollaborationMemoryVersion :one
SELECT * FROM collaboration_memory_version
WHERE issue_id = sqlc.arg(issue_id)::uuid AND version = sqlc.arg(version)::bigint
FOR UPDATE;

-- Metadata only: history pages never carry version text.
-- name: ListCollaborationMemoryVersions :many
SELECT id, issue_id, version, bytes, author_kind, author_user_id, agent_id, run_id, resource_owner_user_id,
    restored_from_version, purged_at, purged_by_user_id, purged_by_admin, created_at
FROM collaboration_memory_version
WHERE issue_id = sqlc.arg(issue_id)::uuid
  AND (sqlc.narg(before_version)::bigint IS NULL OR version < sqlc.narg(before_version)::bigint)
ORDER BY version DESC
LIMIT sqlc.arg(row_limit)::int;

-- No conflict target: either the (issue, version) or the (run, request)
-- unique index losing a race returns no row, and the caller re-reads.
-- name: CreateCollaborationMemoryVersion :one
INSERT INTO collaboration_memory_version (issue_id, version, content, bytes, author_kind, author_user_id,
    agent_id, run_id, resource_owner_user_id, request_id, request_hash, restored_from_version)
VALUES (sqlc.arg(issue_id)::uuid, sqlc.arg(version)::bigint, sqlc.arg(content)::text, sqlc.arg(bytes)::int,
    sqlc.arg(author_kind)::text, sqlc.narg(author_user_id)::uuid, sqlc.narg(agent_id)::uuid, sqlc.narg(run_id)::uuid,
    sqlc.narg(resource_owner_user_id)::uuid, sqlc.narg(request_id)::text, sqlc.narg(request_hash)::text,
    sqlc.narg(restored_from_version)::bigint)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: PurgeCollaborationMemoryVersion :one
UPDATE collaboration_memory_version
SET content = '', purged_at = now(), purged_by_user_id = sqlc.arg(purged_by_user_id)::uuid,
    purged_by_admin = sqlc.arg(purged_by_admin)::boolean, purge_reason = sqlc.arg(purge_reason)::text
WHERE issue_id = sqlc.arg(issue_id)::uuid AND version = sqlc.arg(version)::bigint AND purged_at IS NULL
RETURNING *;

-- name: GetCollaborationMemoryRunWrite :one
SELECT * FROM collaboration_memory_version
WHERE run_id = sqlc.arg(run_id)::uuid AND request_id = sqlc.arg(request_id)::text;

-- Zero means the run wrote no version; real versions start at 1.
-- name: GetCollaborationMemoryRunWrittenVersion :one
SELECT COALESCE(max(version), 0)::bigint AS version
FROM collaboration_memory_version WHERE run_id = sqlc.arg(run_id)::uuid;

-- Written once, in the first-claim transaction that freezes the prompt. Later
-- claims, and runs created before shared context existed, are never relabeled.
-- name: SetCollaborationRunMemoryMode :execrows
UPDATE collaboration_run SET memory_mode = sqlc.arg(memory_mode)::text
WHERE id = sqlc.arg(run_id)::uuid AND memory_mode IS NULL;

-- The run's starting version. It changes only when the recorded base was
-- purged, so a fresh-session retry keeps reading the same document.
-- name: SetCollaborationRunMemoryBase :execrows
UPDATE collaboration_run SET memory_base_version = sqlc.arg(version)::bigint
WHERE id = sqlc.arg(run_id)::uuid AND memory_mode = 'supported';

-- Display names only; agent configuration and runtime identity stay private.
-- name: ListCollaborationMemoryAgentNames :many
SELECT id, name FROM agent WHERE id = ANY(sqlc.arg(agent_ids)::uuid[]) ORDER BY id;
