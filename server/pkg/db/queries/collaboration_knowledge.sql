-- These operations borrow the caller's transaction. Human writers acquire
-- user/member authority first, then parents, target revision, source and sorted
-- candidate locks. They must reauthorize before any body-bearing projection.

-- name: LockKnowledgeWorkspace :one
SELECT id FROM workspace WHERE id = $1 FOR KEY SHARE NOWAIT;

-- name: LockKnowledgeProject :one
SELECT id, workspace_id FROM project WHERE id = $1 AND workspace_id = $2 FOR KEY SHARE NOWAIT;

-- name: LockKnowledgeProjectLead :one
SELECT id, workspace_id, lead_id, lead_type FROM project WHERE id = $1 AND workspace_id = $2 FOR SHARE NOWAIT;

-- name: GetKnowledgeTarget :one
SELECT * FROM collaboration_knowledge_target WHERE target_kind = $1 AND target_id = $2;

-- The caller saves SHOW lock_timeout, sets 25ms for the initialization INSERT,
-- restores the saved value after success and rolls back the whole transaction
-- on 55P03. ON CONFLICT can itself wait and is not a NOWAIT operation.
-- name: SetKnowledgeLockTimeout :one
SELECT set_config('lock_timeout', sqlc.arg(timeout)::text, true)::text;

-- name: GetKnowledgeLockTimeout :one
SELECT current_setting('lock_timeout')::text;

-- Expiry decisions use the same clock as the candidate write predicates.
-- name: GetKnowledgeDatabaseTime :one
SELECT clock_timestamp()::timestamptz AS checked_at;

-- name: InitializeKnowledgeTarget :execrows
INSERT INTO collaboration_knowledge_target (target_kind, target_id, workspace_id)
SELECT sqlc.arg(target_kind)::text, sqlc.arg(target_id)::uuid, sqlc.arg(workspace_id)::uuid
WHERE EXISTS (SELECT 1 FROM workspace WHERE id = sqlc.arg(workspace_id))
  AND ((sqlc.arg(target_kind)::text = 'workspace' AND sqlc.arg(target_id) = sqlc.arg(workspace_id))
    OR (sqlc.arg(target_kind)::text = 'project' AND EXISTS (SELECT 1 FROM project WHERE id = sqlc.arg(target_id) AND workspace_id = sqlc.arg(workspace_id))))
  AND NOT EXISTS (SELECT 1 FROM collaboration_knowledge_candidate
    WHERE target_kind = sqlc.arg(target_kind) AND target_workspace_id = sqlc.arg(workspace_id)
      AND COALESCE(target_project_id, target_workspace_id) = sqlc.arg(target_id) AND published_target_revision IS NOT NULL)
ON CONFLICT (target_kind, target_id) DO NOTHING;

-- name: LockKnowledgeTargetForWrite :one
SELECT * FROM collaboration_knowledge_target WHERE target_kind = $1 AND target_id = $2 FOR UPDATE NOWAIT;

-- name: LockKnowledgeTargetForRead :one
SELECT * FROM collaboration_knowledge_target WHERE target_kind = $1 AND target_id = $2 FOR SHARE NOWAIT;

-- name: AdvanceKnowledgeTarget :one
UPDATE collaboration_knowledge_target SET knowledge_revision = (knowledge_revision + 1)::bigint
WHERE target_kind = $1 AND target_id = $2 AND knowledge_revision = $3 RETURNING *;

-- Only the parent deletion owner calls this, after its final hint is retained.
-- name: DeleteKnowledgeTarget :execrows
DELETE FROM collaboration_knowledge_target WHERE target_kind = $1 AND target_id = $2 AND workspace_id = $3;

-- name: GetKnowledgeCandidate :one
SELECT * FROM collaboration_knowledge_candidate WHERE id = $1;

-- name: GetKnowledgeCandidateRouting :one
SELECT id, lineage_id, source_workspace_id, source_issue_id, target_kind, target_workspace_id, target_project_id
FROM collaboration_knowledge_candidate WHERE id = $1;

-- name: LockKnowledgeCandidates :many
SELECT * FROM collaboration_knowledge_candidate WHERE id = ANY(sqlc.arg(ids)::uuid[]) ORDER BY id FOR UPDATE NOWAIT;

-- name: GetKnowledgeLineageLatestVersion :one
SELECT version FROM collaboration_knowledge_candidate WHERE lineage_id = $1 ORDER BY version DESC LIMIT 1;

-- The lineage root must be locked before allocating a correction. Source and
-- target identity are compared here as well as by the service after its locks.
-- name: CreateKnowledgeCandidate :one
INSERT INTO collaboration_knowledge_candidate (
    id, lineage_id, version, source_workspace_id, source_issue_id,
    target_kind, target_workspace_id, target_project_id, content, summary,
    content_sha256, sources, evidence, applicability, proposed_by_user_id,
    expires_at, supersedes_candidate_id, derived_from_candidate_id
)
SELECT sqlc.arg(id)::uuid,
    COALESCE(previous.lineage_id, sqlc.arg(id)::uuid),
    (COALESCE(previous.version, 0) + 1)::bigint,
    sqlc.arg(source_workspace_id)::uuid, sqlc.arg(source_issue_id)::uuid,
    sqlc.arg(target_kind)::text, sqlc.arg(target_workspace_id)::uuid, sqlc.narg(target_project_id)::uuid,
    sqlc.arg(content)::text, sqlc.arg(summary)::text, sqlc.arg(content_sha256)::text,
    sqlc.arg(sources)::jsonb, sqlc.arg(evidence)::jsonb, sqlc.arg(applicability)::jsonb, sqlc.arg(proposed_by_user_id)::uuid,
    sqlc.narg(expires_at)::timestamptz, sqlc.narg(supersedes_candidate_id)::uuid, sqlc.narg(derived_from_candidate_id)::uuid
FROM (SELECT 1) AS seed
LEFT JOIN collaboration_knowledge_candidate previous ON previous.id = sqlc.narg(supersedes_candidate_id)
WHERE (sqlc.narg(supersedes_candidate_id)::uuid IS NULL AND sqlc.arg(expected_previous_version)::bigint = 0
    OR previous.source_workspace_id = sqlc.arg(source_workspace_id) AND previous.source_issue_id = sqlc.arg(source_issue_id)
      AND previous.target_kind = sqlc.arg(target_kind) AND previous.target_workspace_id = sqlc.arg(target_workspace_id)
      AND previous.target_project_id IS NOT DISTINCT FROM sqlc.narg(target_project_id)::uuid
      AND previous.version = sqlc.arg(expected_previous_version)
      AND NOT EXISTS (SELECT 1 FROM collaboration_knowledge_candidate newer WHERE newer.lineage_id = previous.lineage_id AND newer.version > previous.version))
  AND (sqlc.narg(derived_from_candidate_id)::uuid IS NULL OR EXISTS (
    SELECT 1 FROM collaboration_knowledge_candidate origin WHERE origin.id = sqlc.narg(derived_from_candidate_id)
      AND origin.source_workspace_id = sqlc.arg(source_workspace_id) AND origin.source_issue_id = sqlc.arg(source_issue_id)
      AND origin.state = 'approved' AND origin.target_kind = 'project' AND sqlc.arg(target_kind)::text = 'workspace'))
RETURNING *;

-- name: ApproveKnowledgeCandidate :one
UPDATE collaboration_knowledge_candidate SET state = 'approved', state_revision = (state_revision + 1)::bigint,
    approved_by_user_id = sqlc.arg(actor_id), approved_at = clock_timestamp(),
    last_decided_by_user_id = sqlc.arg(actor_id), last_decided_at = clock_timestamp(),
    published_target_revision = sqlc.arg(target_revision)
WHERE id = sqlc.arg(id) AND state = 'pending' AND state_revision = sqlc.arg(expected_state_revision)
    AND version = sqlc.arg(expected_version) AND content_sha256 = sqlc.arg(expected_content_sha256)
    AND (expires_at IS NULL OR expires_at > clock_timestamp())
RETURNING *;

-- name: RejectKnowledgeCandidate :one
UPDATE collaboration_knowledge_candidate SET state = 'rejected', state_revision = (state_revision + 1)::bigint,
    last_decided_by_user_id = sqlc.arg(actor_id), last_decided_at = clock_timestamp(), last_decision_reason = sqlc.arg(reason)
WHERE id = sqlc.arg(id) AND state = 'pending' AND state_revision = sqlc.arg(expected_state_revision) RETURNING *;

-- name: WithdrawKnowledgeCandidate :one
UPDATE collaboration_knowledge_candidate SET state = 'withdrawn', state_revision = (state_revision + 1)::bigint,
    last_decided_by_user_id = sqlc.arg(actor_id), last_decided_at = clock_timestamp(), last_decision_reason = sqlc.arg(reason)
WHERE id = sqlc.arg(id) AND state IN ('pending', 'approved') AND state_revision = sqlc.arg(expected_state_revision) RETURNING *;

-- name: SupersedeKnowledgeCandidate :one
UPDATE collaboration_knowledge_candidate SET state = 'superseded', state_revision = (state_revision + 1)::bigint,
    last_decided_by_user_id = sqlc.arg(actor_id), last_decided_at = clock_timestamp()
WHERE id = sqlc.arg(id) AND state = 'approved' AND state_revision = sqlc.arg(expected_state_revision) RETURNING *;

-- Source availability and expiry are evaluated before projecting these rows.
-- name: ListKnowledgeSourceCandidates :many
SELECT id, lineage_id, version, source_workspace_id, source_issue_id, target_kind, target_workspace_id, target_project_id,
    summary, content_sha256, proposed_by_user_id, proposed_at, expires_at, state, state_revision, published_target_revision
FROM collaboration_knowledge_candidate
WHERE source_issue_id = sqlc.arg(source_issue_id)
  AND (sqlc.narg(after_time)::timestamptz IS NULL OR (proposed_at, id) > (sqlc.narg(after_time)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY proposed_at, id LIMIT sqlc.arg(page_limit);

-- No ACL filtering here: temporarily unreadable sources must not make room in
-- the global target budget. Authorization belongs in the transaction owner.
-- name: ListKnowledgeActiveTargetCandidates :many
SELECT * FROM collaboration_knowledge_candidate
WHERE target_kind = $1 AND target_workspace_id = $2 AND target_project_id IS NOT DISTINCT FROM sqlc.narg(target_project_id)::uuid
    AND state = 'approved' AND (expires_at IS NULL OR expires_at > sqlc.arg(checked_at)::timestamptz)
ORDER BY id;

-- Source mutation adds only this intent; it never waits on a candidate/target.
-- name: EnqueueKnowledgeScan :execrows
INSERT INTO collaboration_knowledge_invalidation (id, kind, cause_id, scope_kind, scope_id)
VALUES (sqlc.arg(id), 'scan', sqlc.arg(cause_id), sqlc.arg(scope_kind), sqlc.narg(scope_id))
ON CONFLICT (cause_id) WHERE kind = 'scan' DO NOTHING;

-- name: LockNextKnowledgeScan :one
SELECT * FROM collaboration_knowledge_invalidation
WHERE kind = 'scan' AND state = 'pending' AND NOT (id = ANY(sqlc.arg(excluded_ids)::uuid[]))
ORDER BY COALESCE(last_attempted_at, created_at), id LIMIT 1 FOR UPDATE SKIP LOCKED;

-- name: StartKnowledgeScan :one
UPDATE collaboration_knowledge_invalidation SET scan_started_at = clock_timestamp(),
    scan_upper_bound = (SELECT c.id FROM collaboration_knowledge_candidate c ORDER BY c.id DESC LIMIT 1)
WHERE collaboration_knowledge_invalidation.id = $1 AND kind = 'scan' AND state = 'pending' AND scan_started_at IS NULL RETURNING *;

-- No source or candidate row locks/joins: retained routing survives deletion.
-- name: NextKnowledgeScanCandidate :one
SELECT candidate.id, candidate.target_kind, candidate.target_workspace_id, candidate.target_project_id
FROM collaboration_knowledge_candidate candidate
WHERE candidate.published_target_revision IS NOT NULL
    AND candidate.id <= sqlc.arg(upper_bound)::uuid
    AND (sqlc.narg(after_id)::uuid IS NULL OR candidate.id > sqlc.narg(after_id)::uuid)
    AND (sqlc.arg(scope_kind)::text = 'all_sources'
        OR sqlc.arg(scope_kind)::text = 'source_issue' AND candidate.source_issue_id = sqlc.narg(scope_id)::uuid
        OR sqlc.arg(scope_kind)::text = 'source_workspace' AND candidate.source_workspace_id = sqlc.narg(scope_id)::uuid)
ORDER BY candidate.id LIMIT 1;

-- name: AdvanceKnowledgeScan :execrows
UPDATE collaboration_knowledge_invalidation SET scan_cursor = sqlc.arg(cursor), last_attempted_at = clock_timestamp()
WHERE id = sqlc.arg(id) AND kind = 'scan' AND state = 'pending' AND scan_started_at IS NOT NULL
    AND (scan_cursor IS NULL OR scan_cursor < sqlc.arg(cursor)::uuid) AND sqlc.arg(cursor)::uuid <= scan_upper_bound;

-- name: CompleteKnowledgeScan :execrows
UPDATE collaboration_knowledge_invalidation SET state = 'done', completed_at = clock_timestamp(), last_attempted_at = clock_timestamp()
WHERE id = $1 AND kind = 'scan' AND state = 'pending' AND scan_started_at IS NOT NULL;

-- Called only after the failed source-independent transaction has rolled back.
-- name: RecordKnowledgeJobAttempt :execrows
UPDATE collaboration_knowledge_invalidation SET last_attempted_at = clock_timestamp()
WHERE collaboration_knowledge_invalidation.id IN (SELECT j.id FROM collaboration_knowledge_invalidation j WHERE j.id = $1 AND j.state = 'pending' FOR UPDATE SKIP LOCKED);

-- Check under the locked target counter before incrementing for a source cause.
-- name: GetKnowledgeTargetHintForCause :one
SELECT * FROM collaboration_knowledge_invalidation
WHERE kind = 'target_hint' AND cause_id = $1 AND target_kind = $2 AND target_id = $3;

-- name: EnqueueKnowledgeTargetHint :execrows
INSERT INTO collaboration_knowledge_invalidation (id, kind, cause_id, target_kind, target_id, target_workspace_id, knowledge_revision)
VALUES (sqlc.arg(id), 'target_hint', sqlc.arg(cause_id), sqlc.arg(target_kind), sqlc.arg(target_id), sqlc.arg(target_workspace_id), sqlc.arg(knowledge_revision))
ON CONFLICT (cause_id, target_kind, target_id) WHERE kind = 'target_hint' DO NOTHING;

-- A hint that failed earlier in the same pass is excluded until the next pass.
-- name: LockNextKnowledgeTargetHint :one
SELECT * FROM collaboration_knowledge_invalidation
WHERE kind = 'target_hint' AND state = 'pending' AND NOT (id = ANY(sqlc.arg(excluded_ids)::uuid[]))
ORDER BY COALESCE(last_attempted_at, created_at), id LIMIT 1 FOR UPDATE SKIP LOCKED;

-- Publish with the stable id:target first; mark only in its successful commit.
-- name: MarkKnowledgeTargetHintDelivered :execrows
UPDATE collaboration_knowledge_invalidation SET state = 'delivered', completed_at = clock_timestamp(), last_attempted_at = clock_timestamp()
WHERE id = $1 AND kind = 'target_hint' AND state = 'pending';
