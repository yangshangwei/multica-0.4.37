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
-- Files the prepared inputs reference come first, then the newest other
-- deliveries. Issue history therefore never displaces a referenced file, and
-- a referenced file absent from this list is no longer delivered.
SELECT * FROM collaboration_attachment
WHERE issue_id = sqlc.arg(issue_id)::uuid AND delivered_at IS NOT NULL AND discarded_at IS NULL
ORDER BY (id = ANY(sqlc.arg(referenced_ids)::uuid[])) DESC, delivered_at DESC, id
LIMIT sqlc.arg(row_limit)::int;

-- name: CollaborationRunHasPreparationStartEvidence :one
-- An unknown/issued claim or attempt belongs to the original recovery path.
-- Never infer nonlaunch merely from the run's current progress label.
-- The server can assign wakeup provenance to a queued, unclaimed continuation.
-- That metadata is not execution evidence; every other event remains a barrier.
SELECT (
    EXISTS (SELECT 1 FROM collaboration_termination WHERE run_id = sqlc.arg(run_id)::uuid)
    OR EXISTS (SELECT 1 FROM collaboration_attempt WHERE run_id = sqlc.arg(run_id)::uuid)
    OR EXISTS (SELECT 1 FROM collaboration_business_capability WHERE run_id = sqlc.arg(run_id)::uuid)
    OR EXISTS (SELECT 1 FROM collaboration_run_event
               WHERE run_id = sqlc.arg(run_id)::uuid AND kind <> 'wakeup_assigned')
)::boolean AS has_evidence;

-- name: CollaborationAgentHasPreparationSkills :one
SELECT EXISTS (SELECT 1 FROM agent_skill WHERE agent_id = $1 AND enabled)::boolean AS has_skills;
