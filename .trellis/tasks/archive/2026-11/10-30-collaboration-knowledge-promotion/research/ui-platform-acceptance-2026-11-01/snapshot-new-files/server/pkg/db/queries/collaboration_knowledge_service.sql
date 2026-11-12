-- name: ListKnowledgeSourceRows :many
SELECT * FROM collaboration_knowledge_candidate
WHERE source_issue_id = sqlc.arg(source_issue_id)
  AND (sqlc.narg(after_at)::timestamptz IS NULL OR (proposed_at,id) > (sqlc.narg(after_at)::timestamptz,sqlc.narg(after_id)::uuid))
  AND (sqlc.narg(through_at)::timestamptz IS NULL OR (proposed_at,id) <= (sqlc.narg(through_at)::timestamptz,sqlc.narg(through_id)::uuid))
ORDER BY proposed_at,id LIMIT sqlc.arg(row_limit);

-- name: GetKnowledgeSourceHead :one
SELECT proposed_at,id FROM collaboration_knowledge_candidate WHERE source_issue_id=$1 ORDER BY proposed_at DESC,id DESC LIMIT 1;

-- name: GetKnowledgeSourceNextExpiry :one
SELECT min(expires_at)::timestamptz FROM collaboration_knowledge_candidate
WHERE source_issue_id=$1 AND state IN ('pending','approved') AND expires_at>clock_timestamp();

-- name: ListKnowledgeLineageRows :many
SELECT * FROM collaboration_knowledge_candidate WHERE lineage_id=$1 ORDER BY version DESC LIMIT $2;

-- name: GetKnowledgeApprovedLineage :one
SELECT * FROM collaboration_knowledge_candidate WHERE lineage_id=$1 AND state='approved';

-- Visibility is filtered before LIMIT; nonmanager target readers cannot infer
-- inaccessible candidate counts from short or empty pages.
-- name: ListKnowledgeTargetRows :many
SELECT c.* FROM collaboration_knowledge_candidate c
WHERE c.target_kind=sqlc.arg(target_kind) AND c.target_workspace_id=sqlc.arg(workspace_id)
  AND c.target_project_id IS NOT DISTINCT FROM sqlc.narg(project_id)::uuid
  AND c.published_target_revision IS NOT NULL
  AND (sqlc.arg(target_confirmer)::boolean OR EXISTS (
    SELECT 1 FROM issue i JOIN issue_collaboration ic ON ic.issue_id=i.id
    WHERE i.id=c.source_issue_id AND i.workspace_id=c.source_workspace_id
      AND (EXISTS (SELECT 1 FROM member m WHERE m.workspace_id=i.workspace_id AND m.user_id=sqlc.arg(viewer_id))
        OR EXISTS (SELECT 1 FROM issue_participant p WHERE p.issue_id=i.id AND p.user_id=sqlc.arg(viewer_id) AND p.revoked_at IS NULL))))
  AND (sqlc.narg(after_at)::timestamptz IS NULL OR (c.proposed_at,c.id) > (sqlc.narg(after_at)::timestamptz,sqlc.narg(after_id)::uuid))
  AND (sqlc.narg(through_at)::timestamptz IS NULL OR (c.proposed_at,c.id) <= (sqlc.narg(through_at)::timestamptz,sqlc.narg(through_id)::uuid))
ORDER BY c.proposed_at,c.id LIMIT sqlc.arg(row_limit);

-- name: GetKnowledgeTargetHead :one
SELECT c.proposed_at,c.id FROM collaboration_knowledge_candidate c
WHERE c.target_kind=sqlc.arg(target_kind) AND c.target_workspace_id=sqlc.arg(workspace_id) AND c.target_project_id IS NOT DISTINCT FROM sqlc.narg(project_id)::uuid
  AND c.published_target_revision IS NOT NULL
  AND (sqlc.arg(target_confirmer)::boolean OR EXISTS (
    SELECT 1 FROM issue i JOIN issue_collaboration ic ON ic.issue_id=i.id
    WHERE i.id=c.source_issue_id AND i.workspace_id=c.source_workspace_id
      AND (EXISTS (SELECT 1 FROM member m WHERE m.workspace_id=i.workspace_id AND m.user_id=sqlc.arg(viewer_id))
        OR EXISTS (SELECT 1 FROM issue_participant p WHERE p.issue_id=i.id AND p.user_id=sqlc.arg(viewer_id) AND p.revoked_at IS NULL))))
ORDER BY c.proposed_at DESC,c.id DESC LIMIT 1;

-- Expiry itself is private source metadata, unlike the public target revision.
-- Restricted target-only rows must not leak it through the page lease.
-- name: GetKnowledgeTargetNextExpiry :one
SELECT min(c.expires_at)::timestamptz FROM collaboration_knowledge_candidate c
WHERE c.target_kind=sqlc.arg(target_kind) AND c.target_workspace_id=sqlc.arg(workspace_id)
  AND c.target_project_id IS NOT DISTINCT FROM sqlc.narg(project_id)::uuid
  AND c.state='approved' AND c.expires_at>clock_timestamp()
  AND EXISTS (SELECT 1 FROM issue i JOIN issue_collaboration ic ON ic.issue_id=i.id
    WHERE i.id=c.source_issue_id AND i.workspace_id=c.source_workspace_id
      AND (EXISTS (SELECT 1 FROM member m WHERE m.workspace_id=i.workspace_id AND m.user_id=sqlc.arg(viewer_id))
        OR EXISTS (SELECT 1 FROM issue_participant p WHERE p.issue_id=i.id AND p.user_id=sqlc.arg(viewer_id) AND p.revoked_at IS NULL)));

-- Metadata-only reads preserve the external proposal/reference boundary.
-- name: GetKnowledgeWorkspaceIdentity :one
SELECT id FROM workspace WHERE id=$1;

-- name: GetKnowledgeProjectIdentity :one
SELECT id,workspace_id FROM project WHERE id=$1 AND workspace_id=$2;

-- name: GetKnowledgeProjectAuthority :one
SELECT id,workspace_id,lead_id,lead_type FROM project WHERE id=$1 AND workspace_id=$2;

-- An original comment tombstone is checked separately before this read.
-- name: GetKnowledgeCommentVersionEvents :many
SELECT * FROM collaboration_event WHERE issue_id=sqlc.arg(issue_id)
  AND kind IN ('comment_created','comment_edited') AND payload->>'comment_id'=sqlc.arg(comment_id)::text
  AND (payload->>'comment_revision'=sqlc.arg(version)::text OR payload->>'comment_revision'=sqlc.arg(next_version)::text)
ORDER BY event_seq LIMIT 3;

-- C acquires users/credentials, parents, target revisions, Chat, source rows,
-- candidates and finally runtime/task. All these intersections are nonblocking.
-- name: LockKnowledgeUsers :many
SELECT * FROM "user" WHERE id=ANY(sqlc.arg(ids)::uuid[]) ORDER BY id FOR SHARE NOWAIT;

-- name: TryLockKnowledgeMemberFence :one
SELECT pg_try_advisory_xact_lock(hashtext((sqlc.arg(workspace_id)::uuid)::text),hashtext((sqlc.arg(user_id)::uuid)::text))::boolean;

-- name: LockKnowledgeMember :one
SELECT * FROM member WHERE workspace_id=$1 AND user_id=$2 FOR SHARE NOWAIT;

-- name: LockKnowledgeSourceIssue :one
SELECT * FROM issue WHERE id=$1 FOR SHARE NOWAIT;

-- name: LockKnowledgeSourceCollaboration :one
SELECT * FROM issue_collaboration WHERE issue_id=$1 FOR SHARE NOWAIT;

-- name: LockKnowledgeSourceSession :one
SELECT * FROM collaboration_session WHERE issue_id=$1 FOR SHARE NOWAIT;

-- name: LockKnowledgeReadCandidates :many
SELECT id FROM collaboration_knowledge_candidate WHERE id=ANY(sqlc.arg(ids)::uuid[]) ORDER BY id FOR SHARE NOWAIT;

-- name: LockKnowledgeMemorySources :many
SELECT id FROM collaboration_memory_version WHERE issue_id=$1 AND version=ANY(sqlc.arg(versions)::bigint[]) ORDER BY version FOR SHARE NOWAIT;

-- name: LockKnowledgeCommentSources :many
SELECT id FROM comment WHERE issue_id=$1 AND id=ANY(sqlc.arg(ids)::uuid[]) ORDER BY id FOR SHARE NOWAIT;

-- name: LockKnowledgeRunSources :many
SELECT r.id FROM collaboration_run r JOIN collaboration_session s ON s.id=r.session_id
WHERE s.issue_id=$1 AND r.id=ANY(sqlc.arg(ids)::uuid[]) ORDER BY r.id FOR SHARE OF r NOWAIT;

-- name: ListKnowledgeAudienceUsers :many
SELECT p.user_id FROM issue_participant p JOIN "user" u ON u.id=p.user_id
WHERE p.issue_id=$1 AND p.revoked_at IS NULL AND u.disabled_at IS NULL ORDER BY p.user_id LIMIT 1025;

-- Pre-lock discovery must not retrieve candidate bodies or summaries.
-- name: ListKnowledgeTargetLockHints :many
SELECT id,source_workspace_id,source_issue_id,sources FROM collaboration_knowledge_candidate
WHERE target_kind=$1 AND target_workspace_id=$2 AND target_project_id IS NOT DISTINCT FROM sqlc.narg(project_id)::uuid
  AND state='approved' AND (expires_at IS NULL OR expires_at>clock_timestamp()) ORDER BY id LIMIT 17;

-- name: GetKnowledgeCandidateLockHint :one
SELECT id,source_workspace_id,source_issue_id,sources FROM collaboration_knowledge_candidate WHERE id=$1;

-- Potential availability is metadata-only. A positive result still requires
-- the full source/version/hash and current-reader checks after locks.
-- name: KnowledgeSourceIdentityAvailable :one
SELECT EXISTS (SELECT 1 FROM issue i JOIN workspace w ON w.id=i.workspace_id
  JOIN issue_collaboration c ON c.issue_id=i.id JOIN collaboration_session s ON s.issue_id=i.id
  WHERE i.id=$1 AND i.workspace_id=$2)::boolean;

-- name: KnowledgeSourceReferencePotentiallyAvailable :one
SELECT CASE sqlc.arg(kind)::text
  WHEN 'memory' THEN EXISTS (SELECT 1 FROM collaboration_memory_version m WHERE m.issue_id=sqlc.arg(issue_id) AND m.version=sqlc.arg(version)::bigint AND m.purged_at IS NULL)
  WHEN 'comment' THEN EXISTS (SELECT 1 FROM comment c WHERE c.id=sqlc.narg(comment_id)::uuid AND c.issue_id=sqlc.arg(issue_id) AND c.workspace_id=sqlc.arg(workspace_id) AND c.deleted_at IS NULL AND c.revision>=sqlc.arg(version)::bigint)
  WHEN 'run_record' THEN EXISTS (SELECT 1 FROM collaboration_run r JOIN collaboration_session s ON s.id=r.session_id WHERE r.id=sqlc.narg(run_id)::uuid AND s.issue_id=sqlc.arg(issue_id))
  ELSE false END::boolean;

-- Credential-cache hits are not current authority. The original verified hash
-- and actor must still match a live token at this authorization point.
-- name: GetKnowledgePAT :one
SELECT id,user_id,expires_at FROM personal_access_token
WHERE token_hash=$1 AND user_id=$2 AND NOT revoked AND (expires_at IS NULL OR expires_at>clock_timestamp());

-- Human writers hold the actor row first, then this original credential row.
-- name: LockKnowledgePAT :one
SELECT id,user_id,expires_at FROM personal_access_token
WHERE token_hash=$1 AND user_id=$2 AND NOT revoked AND (expires_at IS NULL OR expires_at>clock_timestamp()) FOR SHARE NOWAIT;
