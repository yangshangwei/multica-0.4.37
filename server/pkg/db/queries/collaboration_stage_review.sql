-- Code candidate versions share the stage ledger and root workflow lock.
-- name: CreateCollaborationStageCodeCandidate :one
INSERT INTO collaboration_stage_candidate (candidate_id, issue_id, workspace_id, kind, version,
 parent_candidate_id, content, content_digest, source, lifecycle_epoch, submitted_by, operation_id)
VALUES ($1,$2,$3,'code',$4,$5,$6,$7,$8,$9,$10,$11) RETURNING *;

-- name: GetCollaborationStageCodeCandidate :one
SELECT * FROM collaboration_stage_candidate WHERE issue_id=$1 AND candidate_id=$2 AND kind='code';

-- name: GetCollaborationStageCodeHead :one
SELECT * FROM collaboration_stage_candidate WHERE issue_id=$1 AND kind='code' ORDER BY version DESC LIMIT 1;

-- name: ListCollaborationStageCodeCandidates :many
SELECT * FROM collaboration_stage_candidate WHERE issue_id=$1 AND kind='code' ORDER BY version DESC LIMIT $2;

-- name: CreateCollaborationStageReviewDecision :one
INSERT INTO collaboration_stage_decision (decision_id, issue_id, workspace_id, step, decision, candidate_id,
 candidate_version, content_digest, role, role_revision, policy_revision, lifecycle_epoch, actor_user_id,
 participant_epoch, account_epoch, credential_kind, signer_credential, expires_at, reason, payload, operation_id)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21) RETURNING *;

-- name: ListCollaborationStageReviewDecisions :many
SELECT * FROM collaboration_stage_decision WHERE issue_id=$1 AND candidate_id=$2
 AND step IN ('code_review','qa','release','rollback') ORDER BY created_at DESC, decision_id DESC LIMIT $3;

-- name: GetCollaborationStageReviewHead :one
SELECT * FROM collaboration_stage_decision WHERE issue_id=$1 AND candidate_id=$2 AND step=$3
ORDER BY created_at DESC, decision_id DESC LIMIT 1;

-- name: CreateCollaborationStageRevocation :one
INSERT INTO collaboration_stage_revocation (revocation_id, decision_id, issue_id, workspace_id, actor_user_id, operation_id, reason)
VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING *;

-- name: GetCollaborationStageRevocation :one
SELECT * FROM collaboration_stage_revocation WHERE issue_id=$1 AND decision_id=$2;

-- name: AdvanceCollaborationStage :one
UPDATE collaboration_stage_task SET stage=$2, stage_revision=stage_revision+1, updated_at=clock_timestamp()
WHERE issue_id=$1 AND stage_revision=$3 RETURNING *;

-- name: ResetCollaborationStageOnReopen :exec
UPDATE collaboration_stage_task SET stage='development', stage_revision=stage_revision+1, updated_at=clock_timestamp() WHERE issue_id=$1;

-- name: GetCollaborationStageGitHubSource :one
SELECT pr.*, ins.id AS integration_id, ins.trust_revision
FROM github_pull_request pr
JOIN issue_pull_request link ON link.pull_request_id=pr.id AND NOT link.reference_only
JOIN github_installation ins ON ins.workspace_id=pr.workspace_id AND ins.installation_id=pr.installation_id
WHERE pr.id=$1 AND pr.workspace_id=$2 AND link.issue_id=$3;

-- name: GetCollaborationStageVCSSource :one
SELECT pr.*, c.trust_revision
FROM vcs_pull_request pr
JOIN issue_vcs_pull_request link ON link.pull_request_id=pr.id AND NOT link.reference_only
JOIN vcs_connection c ON c.id=pr.connection_id AND c.workspace_id=pr.workspace_id
WHERE pr.id=$1 AND pr.workspace_id=$2 AND link.issue_id=$3;

-- name: ListCollaborationStageGitHubChecks :many
SELECT * FROM github_pull_request_check_run WHERE pr_id=$1 ORDER BY ordinal;

-- Read-only credential identity for R4 hints; writers use the existing M1
-- credential validator with its live NOWAIT locks before consuming a decision.
-- name: GetCollaborationStageSignerPAT :one
SELECT id, token_hash, expires_at FROM personal_access_token
WHERE id=$1 AND user_id=$2 AND token_hash=$3 AND NOT revoked
 AND (expires_at IS NULL OR expires_at>clock_timestamp());
