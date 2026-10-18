-- The caller first proves the current human owns a resource referenced by the
-- authorized issue or proposal. This is not an account/runtime directory.
-- name: ListCollaborationResourceEnrollments :many
SELECT * FROM collaboration_runtime_enrollment
WHERE runtime_id = sqlc.arg(runtime_id)::uuid
  AND owner_user_id = sqlc.arg(owner_user_id)::uuid
  AND execution_workspace_id = sqlc.arg(execution_workspace_id)::uuid
  AND revoked_at IS NULL
ORDER BY id;
