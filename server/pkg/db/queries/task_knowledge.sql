-- Read every negotiated row in scope in index-ordered keyset pages.
-- A legacy NULL-version row has NULL delivery by the metadata CHECK, so it
-- cannot carry exposure. Every selected row is decoded fail-closed in Go.
-- name: ListTaskKnowledgeHistoryPage :many
SELECT id, agent_id, runtime_id, issue_id, chat_session_id, session_id,
       claim_generation, knowledge_context_version, knowledge_delivery, created_at
FROM agent_task_queue
WHERE agent_id = sqlc.arg(agent_id)::uuid
  AND knowledge_context_version IS NOT NULL
  AND (issue_id = sqlc.narg(issue_id)::uuid OR chat_session_id = sqlc.narg(chat_session_id)::uuid
       OR knowledge_delivery->>'scope_id' = sqlc.arg(scope_id)::text)
  AND (created_at, id) < (sqlc.arg(before_created_at)::timestamptz, sqlc.arg(before_id)::uuid)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size)::int;

-- name: SetTaskKnowledgeDelivery :one
UPDATE agent_task_queue SET knowledge_context_version=1, knowledge_delivery=sqlc.arg(delivery)::jsonb
WHERE id=sqlc.arg(task_id)::uuid AND claim_generation=sqlc.arg(claim_generation)::bigint
RETURNING *;

-- Late authorization locks must never wait behind the finalizer's inverse graph.
-- name: LockTaskKnowledgeTask :one
SELECT * FROM agent_task_queue WHERE id=$1 FOR UPDATE NOWAIT;

-- name: LockTaskKnowledgeRuntime :one
SELECT * FROM agent_runtime WHERE id=$1 FOR SHARE NOWAIT;

-- name: LockTaskKnowledgeAgent :one
SELECT * FROM agent WHERE id=$1 FOR SHARE NOWAIT;

-- name: LockTaskKnowledgeChat :one
SELECT * FROM chat_session WHERE id=$1 FOR SHARE NOWAIT;

-- name: LockTaskKnowledgeDaemonCredential :one
SELECT * FROM daemon_token WHERE token_hash=$1 FOR SHARE NOWAIT;

-- name: LockTaskKnowledgeTaskCredential :one
SELECT * FROM task_token WHERE token_hash=$1 FOR SHARE NOWAIT;

-- name: TryTaskKnowledgeInstallationFence :one
SELECT pg_try_advisory_xact_lock(hashtextextended($1::text,81271465))::boolean;

-- name: LockTaskKnowledgeInstallation :one
SELECT * FROM managed_installation WHERE id=$1 FOR SHARE NOWAIT;

-- name: LockTaskKnowledgeAutopilot :one
SELECT * FROM autopilot WHERE id=$1 FOR SHARE NOWAIT;

-- name: LockTaskKnowledgeAutopilotRun :one
SELECT * FROM autopilot_run WHERE id=$1 FOR SHARE NOWAIT;

-- A legacy/unsupported invocation cannot prove which implicit native session
-- it would choose. Read the complete namespace keys once, then fetch delivery
-- rows in bounded batches. UNION keeps each arm indexable, including immutable
-- runtime/daemon/owner identities after runtime deletion or merge. Go refuses
-- missing or damaged selected rows and stops at the first private exposure.
-- name: ListTaskKnowledgeRuntimeHistoryKeys :many
SELECT history.id, history.created_at
FROM (
    SELECT task.id, task.created_at
    FROM agent_task_queue task
    WHERE task.knowledge_context_version IS NOT NULL
      AND task.runtime_id = sqlc.arg(runtime_id)::uuid
    UNION
    SELECT task.id, task.created_at
    FROM agent a JOIN agent_task_queue task ON task.agent_id = a.id
    WHERE a.runtime_id = sqlc.arg(runtime_id)::uuid
      AND task.knowledge_context_version IS NOT NULL
    UNION
    SELECT task.id, task.created_at
    FROM agent_task_queue task
    WHERE task.knowledge_context_version IS NOT NULL
      AND (task.knowledge_delivery @> jsonb_build_object('claims',jsonb_build_array(jsonb_build_object('descriptor',jsonb_build_object('runtime_id',sqlc.arg(runtime_id)::text))))
        OR task.knowledge_delivery @> jsonb_build_object('claims',jsonb_build_array(jsonb_build_object('daemon_id',sqlc.arg(daemon_id)::text,'descriptor',jsonb_build_object('provider',sqlc.arg(provider)::text))))
        OR task.knowledge_delivery @> jsonb_build_object('claims',jsonb_build_array(jsonb_build_object('runtime_owner_user_id',sqlc.arg(owner_id)::text,'descriptor',jsonb_build_object('provider',sqlc.arg(provider)::text)))))
) history
ORDER BY history.created_at DESC, history.id DESC;

-- Fetch bounded delivery batches by their complete namespace key set.
-- name: GetTaskKnowledgeHistoryRows :many
SELECT id, agent_id, runtime_id, knowledge_context_version, knowledge_delivery
FROM agent_task_queue
WHERE id = ANY(sqlc.arg(ids)::uuid[]);
