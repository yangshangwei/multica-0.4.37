-- Parent-task budget root (F3-S1). Storage-level queries only: the writers
-- that use them hold the locks design section 5 prescribes.

-- name: InsertIssueBudgetRoot :one
INSERT INTO issue_budget_root (id, workspace_id, root_issue_id, limit_ticks, attempt_reserve_ticks, activated_at)
VALUES (@id, @workspace_id, @root_issue_id, sqlc.narg('limit_ticks'), sqlc.narg('attempt_reserve_ticks'), @activated_at)
RETURNING *;

-- name: GetIssueBudgetRootByIssue :one
SELECT * FROM issue_budget_root
WHERE workspace_id = @workspace_id AND root_issue_id = @root_issue_id;

-- name: LockIssueBudgetRoot :one
-- Only claim finalization, the usage endpoint, manual management and the
-- lifecycle reconciliation lock a root, each as the last lock of its
-- transaction; the reconciliation takes no other lock at all. Issue create,
-- move and delete transactions never call this.
SELECT * FROM issue_budget_root
WHERE workspace_id = @workspace_id AND id = @id
FOR UPDATE;

-- name: InsertIssueBudgetMember :one
INSERT INTO issue_budget_member (workspace_id, issue_id, budget_root_id, bound_parent_issue_id, bind_source, binding_version)
VALUES (@workspace_id, @issue_id, @budget_root_id, sqlc.narg('bound_parent_issue_id'), @bind_source, @binding_version)
RETURNING *;

-- name: GetIssueBudgetMember :one
SELECT * FROM issue_budget_member
WHERE workspace_id = @workspace_id AND issue_id = @issue_id;

-- name: InsertIssueBudgetAttempt :one
-- reserved_ticks is NULL for an unsized reservation: the root declared no
-- reserve when the attempt was claimed.
INSERT INTO issue_budget_attempt (
    id, workspace_id, budget_root_id, task_id, claim_generation, runtime_id, agent_id,
    issue_id, context_parent_issue_id, reserved_ticks,
    protocol_version, policy_revision, policy_epoch
) VALUES (
    @id, @workspace_id, @budget_root_id, @task_id, @claim_generation, @runtime_id, @agent_id,
    sqlc.narg('issue_id'), sqlc.narg('context_parent_issue_id'), sqlc.narg('reserved_ticks'),
    sqlc.narg('protocol_version'), sqlc.narg('policy_revision'), sqlc.narg('policy_epoch')
)
RETURNING *;

-- name: InsertIssueBudgetEntry :one
-- The price snapshot pair is set only on a usage entry the server estimated.
INSERT INTO issue_budget_entry (
    workspace_id, budget_root_id, seq, entry_id, kind, attempt_id, invocation_id, scope_id,
    entry_json, entry_digest, receipt, price_snapshot, price_digest,
    source, schema_version, adapter_version, normalizer_version, producer_epoch, payload_digest,
    observed_at, reported_at, received_at, completeness,
    operation_id
) VALUES (
    @workspace_id, @budget_root_id, @seq, @entry_id, @kind, sqlc.narg('attempt_id'),
    sqlc.narg('invocation_id'), sqlc.narg('scope_id'),
    @entry_json, @entry_digest, @receipt, sqlc.narg('price_snapshot'), sqlc.narg('price_digest'),
    @source, @schema_version, sqlc.narg('adapter_version'), sqlc.narg('normalizer_version'),
    sqlc.narg('producer_epoch'), sqlc.narg('payload_digest'),
    sqlc.narg('observed_at'), sqlc.narg('reported_at'), sqlc.narg('received_at'), sqlc.narg('completeness'),
    sqlc.narg('operation_id')
)
RETURNING *;

-- name: ListIssueBudgetEntries :many
-- The whole ledger of one root in replay order, read by a writer that holds
-- the root lock. The (budget_root_id, seq) key can return it already ordered;
-- it never reads another root's entries.
SELECT * FROM issue_budget_entry
WHERE workspace_id = @workspace_id AND budget_root_id = @budget_root_id
ORDER BY seq;

-- name: ListIssueBudgetEntryFormsAfter :many
-- What a replay needs of one root's entries after a seq, in replay order: the
-- stored form and its version. A reader that holds the ledger at a revision
-- asks for the entries after it; a reader that holds nothing asks for the
-- entries after 0, which is all of them. It walks the (budget_root_id, seq)
-- key from that point on and reads no entry before it.
SELECT seq, schema_version, entry_json FROM issue_budget_entry
WHERE workspace_id = @workspace_id AND budget_root_id = @budget_root_id AND seq > @after_seq
ORDER BY seq;

-- name: InsertIssueBudgetOperation :one
INSERT INTO issue_budget_operation (
    id, workspace_id, budget_root_id, scope_kind, scope_id, operation_key, operation_kind,
    intent_digest, actor_type, actor_id, result
) VALUES (
    @id, @workspace_id, @budget_root_id, @scope_kind, @scope_id, @operation_key, @operation_kind,
    @intent_digest, @actor_type, @actor_id, @result
)
RETURNING *;

-- name: InsertIssueBudgetObservation :one
INSERT INTO issue_budget_observation (
    id, workspace_id, budget_root_id, kind, reason, issue_id, task_id, claim_generation, attempt_id, detail
) VALUES (
    @id, @workspace_id, @budget_root_id, @kind, @reason,
    sqlc.narg('issue_id'), sqlc.narg('task_id'), sqlc.narg('claim_generation'), sqlc.narg('attempt_id'), @detail
)
RETURNING *;

-- name: GetIssueBudgetRoot :one
-- An unlocked read by the root's own id. The workspace is part of the key on
-- purpose: a root id from another workspace finds nothing.
SELECT * FROM issue_budget_root
WHERE workspace_id = @workspace_id AND id = @id;

-- name: UpdateIssueBudgetRootPolicy :one
-- Manual management only, under the root lock. Every change of the admission
-- state, the mode, the limit or the declared reserve is one new policy
-- revision; no amount moves.
UPDATE issue_budget_root
SET admission_state = @admission_state,
    mode = @mode,
    limit_ticks = sqlc.narg('limit_ticks'),
    attempt_reserve_ticks = sqlc.narg('attempt_reserve_ticks'),
    policy_epoch = @policy_epoch,
    policy_revision = policy_revision + 1,
    updated_at = now()
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: UpdateIssueBudgetRootLedger :one
-- The writer that appended entries under the root lock moves the root to the
-- seq of its last entry and stores the projection of the ledger at that
-- revision. No policy column moves. It finds no row when the root is not at
-- the revision the append started from.
UPDATE issue_budget_root
SET ledger_revision = @ledger_revision,
    actual_known_ticks = @actual_known_ticks,
    estimated_ticks = @estimated_ticks,
    reserved_ticks = @reserved_ticks,
    unknown_hold_ticks = @unknown_hold_ticks,
    overrun_ticks = @overrun_ticks,
    unknown_count = @unknown_count,
    updated_at = now()
WHERE workspace_id = @workspace_id AND id = @id AND ledger_revision = @from_ledger_revision
RETURNING *;

-- name: InsertIssueBudgetMembers :execrows
-- The members one walk binds, in one statement: the writer holds the
-- workspace fence, so the fewer round trips the shorter it is held.
INSERT INTO issue_budget_member (workspace_id, issue_id, budget_root_id, bound_parent_issue_id, bind_source, binding_version)
SELECT @workspace_id, i.issue_id, @budget_root_id, p.bound_parent_issue_id, @bind_source, @binding_version
FROM unnest(@issue_ids::uuid[]) WITH ORDINALITY AS i(issue_id, ordinality)
JOIN unnest(@bound_parent_issue_ids::uuid[]) WITH ORDINALITY AS p(bound_parent_issue_id, ordinality)
  ON p.ordinality = i.ordinality;

-- name: TombstoneIssueBudgetMember :execrows
-- Issue deletion marks the member row and leaves every other budget row,
-- the root row included, alone. For an issue outside every root this is one
-- lookup on the (workspace_id, issue_id) key that updates nothing.
UPDATE issue_budget_member
SET tombstoned_at = now()
WHERE workspace_id = @workspace_id AND issue_id = @issue_id AND tombstoned_at IS NULL;

-- name: ListIssueBudgetDescendants :many
-- Every issue below @issue_id, with the member row it already has, if any.
-- No depth limit. An issue has one parent, so the only way this walk can meet
-- an issue twice is to come back to the one it started from: that row is
-- returned with cycle = true and is not expanded, which is what ends a walk
-- through a parent loop.
WITH RECURSIVE walk AS (
    SELECT i.id, i.parent_issue_id, 1 AS depth, i.id = @issue_id::uuid AS cycle
    FROM issue i
    WHERE i.workspace_id = @workspace_id AND i.parent_issue_id = @issue_id::uuid
    UNION ALL
    SELECT c.id, c.parent_issue_id, w.depth + 1, c.id = @issue_id::uuid
    FROM walk w
    JOIN issue c ON c.parent_issue_id = w.id AND c.workspace_id = @workspace_id
    WHERE NOT w.cycle
)
SELECT w.id AS issue_id,
       w.parent_issue_id::uuid AS parent_issue_id,
       w.depth::integer AS depth,
       w.cycle::boolean AS cycle,
       m.budget_root_id AS member_budget_root_id
FROM walk w
LEFT JOIN issue_budget_member m ON m.workspace_id = @workspace_id AND m.issue_id = w.id
ORDER BY w.depth, w.id;

-- name: LockIssueBudgetBinding :exec
-- The binder's side of the workspace binding lock: exclusive, and waited for.
-- A transaction that gives an existing issue its first member row (enabling a
-- root, moving an unbound issue under a member) takes it after the
-- iteration:<workspace> fence and before any root row lock, and holds it until
-- it ends. The key is not the fence's.
SELECT pg_advisory_xact_lock(hashtextextended('issue_budget_binding:' || sqlc.arg('workspace_id')::uuid::text, 0));

-- name: TryLockIssueBudgetBindingShared :one
-- The claim's side of the same lock: shared, and never waited for. false means
-- a binder holds the lock or is queued for it.
SELECT pg_try_advisory_xact_lock_shared(hashtextextended('issue_budget_binding:' || sqlc.arg('workspace_id')::uuid::text, 0))::boolean AS acquired;

-- name: GetIssueBudgetTransactionIsolation :one
-- The isolation level of the calling transaction, in PostgreSQL's own words
-- ('read committed', 'repeatable read', ...).
SELECT current_setting('transaction_isolation')::text AS isolation_level;

-- name: ListIssueBudgetInFlightTasks :many
-- Tasks a daemon already holds for the given issues: claimed (dispatched),
-- held for a local directory, or running. A task that is still queued or
-- deferred is claimed later and attributed then. A plain read: it locks no
-- task row.
--
-- A task with an issue belongs to that issue's workspace, whichever workspace
-- its agent and runtime are in: the member row it is attributed through is
-- keyed by the issue.
--
-- A quick-create task has no issue of its own and is found through the parent
-- saved in its context. That branch only proposes candidates; whether a task
-- is a quick-create task is decided by service.quickCreateTaskContext on the
-- link columns and the context returned here. It keeps the agent join: without
-- it the planner can only walk the whole status index.
SELECT t.id AS task_id,
       t.claim_generation,
       t.status,
       t.issue_id::uuid AS issue_id,
       false AS quick_create,
       NULL::uuid AS chat_session_id,
       NULL::uuid AS autopilot_run_id,
       NULL::jsonb AS context
FROM agent_task_queue t
JOIN issue i ON i.id = t.issue_id
WHERE i.workspace_id = @workspace_id
  AND t.status IN ('dispatched', 'waiting_local_directory', 'running')
  AND t.issue_id = ANY(@issue_ids::uuid[])
UNION ALL
SELECT t.id AS task_id,
       t.claim_generation,
       t.status,
       scope.issue_id AS issue_id,
       true AS quick_create,
       t.chat_session_id,
       t.autopilot_run_id,
       t.context
FROM agent_task_queue t
JOIN agent a ON a.id = t.agent_id
JOIN unnest(@issue_ids::uuid[]) AS scope(issue_id)
  ON scope.issue_id::text = lower(t.context->>'parent_issue_id')
WHERE a.workspace_id = @workspace_id
  AND t.status IN ('dispatched', 'waiting_local_directory', 'running')
  AND t.issue_id IS NULL
  AND t.context->>'type' = 'quick_create'
ORDER BY task_id;

-- name: GetIssueBudgetOperationByKey :one
-- The committed operation behind an original key, or no row when nothing was
-- committed under it.
SELECT * FROM issue_budget_operation
WHERE workspace_id = @workspace_id
  AND scope_kind = @scope_kind
  AND scope_id = @scope_id
  AND operation_key = @operation_key;

-- name: ListIssueBudgetObservations :many
-- The observations of one root, oldest first. An in-flight gap counts as
-- covered once an attempt exists for the same task and claim generation: the
-- claim was attributed after all. That is judged here, when read; observation
-- rows are never updated.
SELECT sqlc.embed(o),
       (o.reason IN ('in_flight_legacy', 'in_flight_at_bind')
        AND o.task_id IS NOT NULL
        AND o.claim_generation IS NOT NULL
        AND EXISTS (
            SELECT 1 FROM issue_budget_attempt a
            WHERE a.task_id = o.task_id AND a.claim_generation = o.claim_generation
              AND a.workspace_id = o.workspace_id
        ))::boolean AS covered
FROM issue_budget_observation o
WHERE o.workspace_id = @workspace_id AND o.budget_root_id = @budget_root_id
ORDER BY o.created_at, o.id;

-- name: TryLockIssueBudgetBindingSharedForIssue :one
-- The claim of an ordinary task: the workspace of its issue, read by primary
-- key without a row lock, and the shared try lock keyed by that workspace, in
-- one statement. No row means the issue is gone, and no lock was asked for.
-- The member row is never read here: under READ COMMITTED a statement has one
-- snapshot, and the member read has to see what committed before the lock
-- was granted.
SELECT i.workspace_id,
       pg_try_advisory_xact_lock_shared(hashtextextended('issue_budget_binding:' || i.workspace_id::text, 0))::boolean AS acquired
FROM issue i
WHERE i.id = @issue_id;

-- name: ListIssueBudgetOpenAttemptsOfTask :many
-- The earlier claims of one task whose attempt is still open, oldest first.
-- Called under the root lock by the claim that supersedes them.
SELECT * FROM issue_budget_attempt
WHERE task_id = @task_id AND claim_generation < @claim_generation
  AND workspace_id = @workspace_id AND budget_root_id = @budget_root_id
  AND state = 'open'
ORDER BY claim_generation;

-- name: CloseIssueBudgetAttempt :execrows
-- Moves an open attempt to the state that ends it. An attempt leaves 'open'
-- once: a row that is not open any more is not matched.
UPDATE issue_budget_attempt
SET state = @state, updated_at = now()
WHERE workspace_id = @workspace_id AND id = @id AND state = 'open';

-- name: CountIssueBudgetUncoveredGaps :many
-- The part of a root's unknown set that is not in its ledger: the coverage
-- gaps no attempt has covered, counted by reason. Every coverage gap counts
-- unless it is covered, whatever its reason; a reason added later counts
-- without a change here. Covered is judged as ListIssueBudgetObservations
-- judges it.
SELECT o.reason, count(*)::bigint AS gap_count
FROM issue_budget_observation o
WHERE o.workspace_id = @workspace_id AND o.budget_root_id = @budget_root_id
  AND o.kind = 'coverage_gap'
  AND NOT (o.reason IN ('in_flight_legacy', 'in_flight_at_bind')
           AND o.task_id IS NOT NULL
           AND o.claim_generation IS NOT NULL
           AND EXISTS (
               SELECT 1 FROM issue_budget_attempt a
               WHERE a.task_id = o.task_id AND a.claim_generation = o.claim_generation
                 AND a.workspace_id = o.workspace_id
           ))
GROUP BY o.reason
ORDER BY o.reason;

-- name: ListIssueBudgetOpenAttemptsOfRoot :many
-- The open attempts of one root, each with what its task row says now. Called
-- under the root lock by the lifecycle reconciliation. The task is read
-- without a lock: the reconciliation never waits for a task row. task_exists
-- is false when the row is gone; the other task columns are then empty.
-- task_error is read because one failure reason, 'timeout', has two writers
-- that only the error text tells apart.
SELECT a.id, a.workspace_id, a.task_id, a.claim_generation,
       (t.id IS NOT NULL)::boolean AS task_exists,
       COALESCE(t.status, '')::text AS task_status,
       COALESCE(t.failure_reason, '')::text AS task_failure_reason,
       COALESCE(t.error, '')::text AS task_error,
       COALESCE(t.claim_generation, 0)::bigint AS task_claim_generation
FROM issue_budget_attempt a
LEFT JOIN agent_task_queue t ON t.id = a.task_id
WHERE a.workspace_id = @workspace_id AND a.budget_root_id = @budget_root_id
  AND a.state = 'open'
ORDER BY a.id;

-- name: ListIssueBudgetRootsWithEndedAttempts :many
-- The roots the lifecycle reconciliation has work for: a root that is not off
-- and has an open attempt whose task reached a final status or whose task row
-- is gone. In root order, at most max_roots of them; a root that is
-- reconciled drops out, so the next call returns the ones after it.
-- A plain read. It takes no lock and is not a promise: the reconciliation
-- looks at the mode and at the attempts again under the root lock.
-- What counts as ended here has to be what issueBudgetAttemptEndOf closes an
-- attempt for. A root this statement finds and that function closes nothing
-- of would be found again by every call.
-- Reads the open attempts through issue_budget_attempt_open_idx, so the
-- cost follows the attempts that are open and not the size of the table.
SELECT DISTINCT r.id, r.workspace_id
FROM issue_budget_attempt a
JOIN issue_budget_root r ON r.id = a.budget_root_id AND r.workspace_id = a.workspace_id
LEFT JOIN agent_task_queue t ON t.id = a.task_id
WHERE a.state = 'open'
  AND r.mode <> 'off'
  AND (t.id IS NULL OR t.status IN ('completed', 'failed', 'cancelled'))
ORDER BY r.id
LIMIT @max_roots;
