-- PAY-241 S2 stage approvals. Policy revisions, role revisions, candidates and
-- decisions are immutable facts; the task row and child mapping are the only
-- mutable heads. Every reader scopes by issue/workspace.

-- name: GetProjectStagePolicy :one
SELECT * FROM project_stage_policy WHERE workspace_id=$1 AND project_id=$2 ORDER BY revision DESC LIMIT 1;

-- The newest disabling revision (0 when none). A confirmation older than or
-- equal to it belongs to an earlier enabled period.
-- name: GetProjectStagePolicyLastDisabled :one
SELECT COALESCE(MAX(revision), 0)::bigint FROM project_stage_policy
WHERE workspace_id=$1 AND project_id=$2 AND NOT enabled;

-- name: GetProjectStagePolicyByRequest :one
SELECT * FROM project_stage_policy
WHERE workspace_id=$1 AND project_id=$2 AND created_by=$3 AND request_id=$4;

-- name: CreateProjectStagePolicy :one
INSERT INTO project_stage_policy (workspace_id, project_id, revision, contract, enabled, reason, created_by, request_id, request_digest)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING *;

-- name: GetCollaborationStageRoot :one
SELECT i.id, i.workspace_id, i.project_id, c.lifecycle_epoch
FROM issue i JOIN issue_collaboration c ON c.issue_id=i.id WHERE i.id=$1;

-- Child-side fences on the root lifecycle. The blocking form is taken before
-- the child row lock (root before child, the same order as approval); the
-- NOWAIT form is for callers that already hold the child row.
-- name: LockCollaborationStageRootShared :one
SELECT lifecycle_epoch FROM issue_collaboration WHERE issue_id=$1 FOR SHARE;

-- name: TryLockCollaborationStageRootShared :one
SELECT lifecycle_epoch FROM issue_collaboration WHERE issue_id=$1 FOR SHARE NOWAIT;

-- name: EnsureCollaborationStageTask :exec
INSERT INTO collaboration_stage_task (issue_id, workspace_id) VALUES ($1, $2)
ON CONFLICT (issue_id) DO NOTHING;

-- name: LockCollaborationStageTask :one
SELECT * FROM collaboration_stage_task WHERE issue_id=$1 FOR UPDATE;

-- name: GetCollaborationStageTask :one
SELECT * FROM collaboration_stage_task WHERE issue_id=$1;

-- name: SetCollaborationStageRoleRevision :one
UPDATE collaboration_stage_task SET role_revision=sqlc.arg(role_revision)::bigint, updated_at=clock_timestamp()
WHERE issue_id=sqlc.arg(issue_id)::uuid AND role_revision=sqlc.arg(role_revision)::bigint - 1 RETURNING *;

-- name: SetCollaborationStageScope :one
UPDATE collaboration_stage_task SET scope_revision=scope_revision+1,
    scope_decision_id=sqlc.arg(decision_id)::uuid, updated_at=clock_timestamp()
WHERE issue_id=sqlc.arg(issue_id)::uuid AND scope_revision=sqlc.arg(expected_scope_revision)::bigint RETURNING *;

-- name: CreateCollaborationStageRole :one
INSERT INTO collaboration_stage_role (issue_id, workspace_id, revision, business_owner_user_id, implementer_user_id,
    product_approver_user_id, code_reviewer_user_id, qa_approver_user_id, release_approver_user_id,
    policy_revision, lifecycle_epoch, configured_by, operation_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13) RETURNING *;

-- name: GetCollaborationStageRole :one
SELECT * FROM collaboration_stage_role WHERE issue_id=$1 AND revision=$2;

-- name: ListCollaborationStageParticipants :many
SELECT user_id, epoch FROM issue_participant WHERE issue_id=$1 AND revoked_at IS NULL ORDER BY user_id;

-- name: CreateCollaborationStageCandidate :one
INSERT INTO collaboration_stage_candidate (candidate_id, issue_id, workspace_id, kind, proposal_id, version,
    parent_candidate_id, content, content_digest, source, lifecycle_epoch, submitted_by, operation_id)
VALUES (sqlc.arg(candidate_id)::uuid, sqlc.arg(issue_id)::uuid, sqlc.arg(workspace_id)::uuid, 'proposal',
    sqlc.arg(proposal_id)::uuid, sqlc.arg(version)::bigint, sqlc.narg(parent_candidate_id)::uuid,
    sqlc.arg(content)::jsonb, sqlc.arg(content_digest)::text, sqlc.arg(source)::jsonb,
    sqlc.arg(lifecycle_epoch)::bigint, sqlc.arg(submitted_by)::uuid, sqlc.arg(operation_id)::uuid)
RETURNING *;

-- name: GetCollaborationStageProposalHead :one
SELECT * FROM collaboration_stage_candidate
WHERE issue_id=$1 AND proposal_id=$2 ORDER BY version DESC LIMIT 1;

-- name: ListCollaborationStageProposalVersions :many
SELECT * FROM collaboration_stage_candidate
WHERE issue_id=$1 AND proposal_id=$2 ORDER BY version DESC LIMIT $3;

-- Proposal ids are time-ordered (UUIDv7), so the id is the keyset.
-- name: ListCollaborationStageProposalHeads :many
SELECT sqlc.embed(c),
       (SELECT min(v.created_at) FROM collaboration_stage_candidate v
        WHERE v.issue_id=c.issue_id AND v.proposal_id=c.proposal_id)::timestamptz AS proposal_created_at
FROM collaboration_stage_candidate c
WHERE c.issue_id=sqlc.arg(issue_id)::uuid
  AND c.version=(SELECT max(v.version) FROM collaboration_stage_candidate v
                 WHERE v.issue_id=c.issue_id AND v.proposal_id=c.proposal_id)
  AND (sqlc.narg(before_proposal_id)::uuid IS NULL OR c.proposal_id < sqlc.narg(before_proposal_id)::uuid)
ORDER BY c.proposal_id DESC LIMIT sqlc.arg(row_limit)::int;

-- name: CreateCollaborationStageDecision :one
INSERT INTO collaboration_stage_decision (decision_id, issue_id, workspace_id, step, decision, proposal_id, candidate_id,
    candidate_version, content_digest, role, role_revision, policy_revision, lifecycle_epoch, actor_user_id,
    participant_epoch, account_epoch, credential_kind, reason, payload, operation_id)
VALUES (sqlc.arg(decision_id)::uuid, sqlc.arg(issue_id)::uuid, sqlc.arg(workspace_id)::uuid, sqlc.arg(step)::text,
    sqlc.arg(decision)::text, sqlc.narg(proposal_id)::uuid, sqlc.narg(candidate_id)::uuid,
    sqlc.narg(candidate_version)::bigint, sqlc.narg(content_digest)::text, 'product_approver',
    sqlc.arg(role_revision)::bigint, sqlc.arg(policy_revision)::bigint, sqlc.arg(lifecycle_epoch)::bigint,
    sqlc.arg(actor_user_id)::uuid, sqlc.arg(participant_epoch)::bigint, sqlc.arg(account_epoch)::bigint,
    sqlc.arg(credential_kind)::text, sqlc.arg(reason)::text, sqlc.arg(payload)::jsonb, sqlc.arg(operation_id)::uuid)
RETURNING *;

-- name: GetCollaborationStageDecision :one
SELECT * FROM collaboration_stage_decision WHERE decision_id=$1 AND issue_id=$2;

-- name: GetCollaborationStageCandidateDecision :one
SELECT * FROM collaboration_stage_decision WHERE candidate_id=$1 AND issue_id=$2 AND step='proposal';

-- name: ListCollaborationStageProposalDecisions :many
SELECT * FROM collaboration_stage_decision
WHERE issue_id=$1 AND proposal_id=$2 AND step='proposal' ORDER BY created_at, decision_id;

-- name: CreateCollaborationStageChild :one
INSERT INTO collaboration_stage_child (child_issue_id, root_issue_id, workspace_id, origin, proposal_id, candidate_id,
    ordinal, decision_id, lifecycle_epoch)
VALUES ($1, $2, $3, 'materialized', $4, $5, $6, $7, $8) RETURNING *;

-- A confirmation keeps the child's identity and origin; it only refreshes the
-- lifecycle of the approval. A child mapped to another root is not adopted.
-- The confirmed code version replaces the previous one; an explicit "none"
-- clears it.
-- name: ConfirmCollaborationStageChild :one
INSERT INTO collaboration_stage_child (child_issue_id, root_issue_id, workspace_id, origin, decision_id, lifecycle_epoch, scope_decision_id,
    code_provider, code_pull_request_id, code_head_sha)
VALUES (sqlc.arg(child_issue_id)::uuid, sqlc.arg(root_issue_id)::uuid, sqlc.arg(workspace_id)::uuid, 'confirmed',
    sqlc.arg(decision_id)::uuid, sqlc.arg(lifecycle_epoch)::bigint, sqlc.arg(decision_id)::uuid,
    sqlc.narg(code_provider)::text, sqlc.narg(code_pull_request_id)::uuid, sqlc.narg(code_head_sha)::text)
ON CONFLICT (child_issue_id) DO UPDATE SET lifecycle_epoch=EXCLUDED.lifecycle_epoch,
    scope_decision_id=EXCLUDED.scope_decision_id, code_provider=EXCLUDED.code_provider,
    code_pull_request_id=EXCLUDED.code_pull_request_id, code_head_sha=EXCLUDED.code_head_sha, updated_at=clock_timestamp()
WHERE collaboration_stage_child.root_issue_id=EXCLUDED.root_issue_id
RETURNING *;

-- name: GetCollaborationStageChild :one
SELECT * FROM collaboration_stage_child WHERE child_issue_id=$1;

-- name: BumpCollaborationStageChildSemanticVersion :execrows
UPDATE collaboration_stage_child SET semantic_version=semantic_version+1, updated_at=clock_timestamp()
WHERE child_issue_id=$1;

-- name: ListCollaborationStageProposalChildren :many
SELECT m.ordinal, m.child_issue_id, i.number, i.title, i.parent_issue_id
FROM collaboration_stage_child m JOIN issue i ON i.id=m.child_issue_id
WHERE m.root_issue_id=sqlc.arg(root_issue_id)::uuid AND m.proposal_id=sqlc.arg(proposal_id)::uuid AND m.origin='materialized'
ORDER BY m.ordinal;

-- Root before child, NOWAIT: a busy ordinary writer makes the whole workflow
-- attempt roll back and retry instead of waiting while root locks are held.
-- name: LockCollaborationStageScopeChildren :many
SELECT i.id, i.workspace_id, i.parent_issue_id, i.title, i.description, i.admission_status,
       EXISTS (SELECT 1 FROM issue_collaboration c WHERE c.issue_id=i.id)::boolean AS collaboration
FROM issue i WHERE i.id=ANY(sqlc.arg(issue_ids)::uuid[]) ORDER BY i.id FOR SHARE OF i NOWAIT;

-- Ordinary descendants whose nearest collaboration ancestor is this root (the
-- walk stops at a nested collaboration issue), plus issues still mapped to this
-- root after leaving the subtree (depth 0). The caller bounds the page.
-- name: ListCollaborationStageViewChildren :many
WITH RECURSIVE tree AS (
    SELECT i.id, 1 AS depth FROM issue i
    WHERE i.workspace_id=sqlc.arg(workspace_id)::uuid AND i.parent_issue_id=sqlc.arg(root_issue_id)::uuid
      AND NOT EXISTS (SELECT 1 FROM issue_collaboration c WHERE c.issue_id=i.id)
    UNION ALL
    SELECT i.id, t.depth+1 FROM tree t JOIN issue i ON i.parent_issue_id=t.id
    WHERE i.workspace_id=sqlc.arg(workspace_id)::uuid AND t.depth < sqlc.arg(max_depth)::int
      AND NOT EXISTS (SELECT 1 FROM issue_collaboration c WHERE c.issue_id=i.id)
), scope AS (
    SELECT t.id, min(t.depth)::int AS depth FROM tree t GROUP BY t.id
    UNION ALL
    SELECT s.child_issue_id, NULL::int FROM collaboration_stage_child s
    WHERE s.root_issue_id=sqlc.arg(root_issue_id)::uuid AND s.child_issue_id NOT IN (SELECT t.id FROM tree t)
)
SELECT i.id, i.number, i.title, i.description, i.creator_type, i.parent_issue_id, COALESCE(scope.depth, 0)::int AS depth,
       m.origin, m.proposal_id, m.ordinal, m.lifecycle_epoch AS approved_lifecycle_epoch, m.semantic_version,
       m.code_provider, m.code_pull_request_id, m.code_head_sha
FROM scope JOIN issue i ON i.id=scope.id
LEFT JOIN collaboration_stage_child m ON m.child_issue_id=i.id AND m.root_issue_id=sqlc.arg(root_issue_id)::uuid
WHERE i.workspace_id=sqlc.arg(workspace_id)::uuid
  AND i.admission_status IN ('not_required','accepted')
  AND NOT EXISTS (SELECT 1 FROM issue_collaboration c WHERE c.issue_id=i.id)
ORDER BY i.number ASC LIMIT sqlc.arg(row_limit)::int;

-- The governing root of an ordinary issue is its nearest collaboration
-- ancestor, the issue itself included. The depth bound also ends a parent
-- cycle; `unresolved` reports a chain cut by the bound, which callers that gate
-- machine work must treat as not provably outside a managed root.
-- name: GetCollaborationStageGoverningRoot :one
WITH RECURSIVE chain AS (
    SELECT i.id, i.parent_issue_id, 0 AS depth,
           EXISTS (SELECT 1 FROM issue_collaboration c WHERE c.issue_id=i.id) AS collaboration
    FROM issue i WHERE i.id=sqlc.arg(issue_id)::uuid
    UNION ALL
    SELECT p.id, p.parent_issue_id, c.depth+1,
           EXISTS (SELECT 1 FROM issue_collaboration x WHERE x.issue_id=p.id)
    FROM chain c JOIN issue p ON p.id=c.parent_issue_id
    WHERE NOT c.collaboration AND c.depth < sqlc.arg(max_depth)::int
)
SELECT COALESCE((SELECT c.id FROM chain c WHERE c.collaboration LIMIT 1), '00000000-0000-0000-0000-000000000000'::uuid)::uuid AS root_issue_id,
       EXISTS (SELECT 1 FROM chain c WHERE NOT c.collaboration AND c.depth=sqlc.arg(max_depth)::int AND c.parent_issue_id IS NOT NULL)::boolean AS unresolved;

-- Server-observed pull requests actually linked to these issues. A link that
-- only mentions the issue (reference_only) is not a code version of it.
-- name: ListCollaborationStageChildPullRequests :many
SELECT l.issue_id, 'github'::text AS provider, pr.id AS pull_request_id, pr.head_sha
FROM issue_pull_request l JOIN github_pull_request pr ON pr.id=l.pull_request_id
WHERE l.issue_id=ANY(sqlc.arg(issue_ids)::uuid[]) AND NOT l.reference_only
  AND pr.workspace_id=sqlc.arg(workspace_id)::uuid AND pr.head_sha ~ '^([0-9a-f]{40}|[0-9a-f]{64})$'
UNION ALL
SELECT v.issue_id, 'vcs'::text AS provider, vp.id AS pull_request_id, vp.head_sha
FROM issue_vcs_pull_request v JOIN vcs_pull_request vp ON vp.id=v.pull_request_id
WHERE v.issue_id=ANY(sqlc.arg(issue_ids)::uuid[]) AND NOT v.reference_only
  AND vp.workspace_id=sqlc.arg(workspace_id)::uuid AND vp.head_sha ~ '^([0-9a-f]{40}|[0-9a-f]{64})$'
ORDER BY 1, 2, 3;

-- name: GetCollaborationStageRunSource :one
SELECT r.id, r.agent_id FROM collaboration_run r JOIN collaboration_session s ON s.id=r.session_id
WHERE r.id=sqlc.arg(run_id)::uuid AND s.issue_id=sqlc.arg(issue_id)::uuid;

-- A coarse read-side presence check; writers and activation use the exact
-- readiness predicate shared with migration 819.
-- name: CollaborationStageSchemaReady :one
SELECT (to_regclass('project_stage_policy') IS NOT NULL AND to_regclass('collaboration_stage_task') IS NOT NULL
    AND to_regclass('collaboration_stage_role') IS NOT NULL AND to_regclass('collaboration_stage_candidate') IS NOT NULL
    AND to_regclass('collaboration_stage_decision') IS NOT NULL AND to_regclass('collaboration_stage_child') IS NOT NULL)::boolean;
