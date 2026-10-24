-- name: RegisterCollaborationRuntimeCapabilities :one
UPDATE agent_runtime SET collaboration_capabilities = sqlc.arg(capabilities)::jsonb
WHERE id = sqlc.arg(runtime_id)::uuid AND collaboration_config_epoch = sqlc.arg(expected_epoch)::bigint
RETURNING *;

-- name: FreezeCollaborationPreparedRunPrompt :one
UPDATE collaboration_run SET prompt_snapshot = sqlc.arg(prompt_snapshot)::jsonb,
    preparation_snapshot = sqlc.arg(preparation_snapshot)::jsonb
WHERE id = sqlc.arg(run_id)::uuid AND prompt_snapshot IS NULL AND preparation_snapshot IS NULL
    AND authorized_attempt_seq = 0 AND settled_at IS NULL
RETURNING *;

-- name: ListCollaborationPreparationAttachments :many
-- One beyond the manifest bound distinguishes overflow from a complete list.
SELECT * FROM collaboration_attachment
WHERE issue_id = $1 AND delivered_at IS NOT NULL AND discarded_at IS NULL
ORDER BY id LIMIT 33;

-- name: CollaborationRunHasPreparationStartEvidence :one
-- An unknown/issued claim or attempt belongs to the original recovery path.
-- Never infer nonlaunch merely from the run's current progress label.
SELECT (
    EXISTS (SELECT 1 FROM collaboration_termination WHERE run_id = sqlc.arg(run_id)::uuid)
    OR EXISTS (SELECT 1 FROM collaboration_attempt WHERE run_id = sqlc.arg(run_id)::uuid)
    OR EXISTS (SELECT 1 FROM collaboration_business_capability WHERE run_id = sqlc.arg(run_id)::uuid)
    OR EXISTS (SELECT 1 FROM collaboration_run_event WHERE run_id = sqlc.arg(run_id)::uuid)
)::boolean AS has_evidence;

-- name: CollaborationAgentHasPreparationSkills :one
SELECT EXISTS (SELECT 1 FROM agent_skill WHERE agent_id = $1 AND enabled)::boolean AS has_skills;
