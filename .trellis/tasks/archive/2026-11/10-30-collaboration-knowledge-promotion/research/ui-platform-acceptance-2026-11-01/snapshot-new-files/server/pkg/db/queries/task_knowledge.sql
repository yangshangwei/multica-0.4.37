-- Read immutable consumer history without filtering away old runtime identities.
-- Any malformed negotiated row in the selected scope must fail closed in Go.
-- name: ListTaskKnowledgeHistory :many
SELECT id, agent_id, runtime_id, issue_id, chat_session_id, session_id,
       claim_generation, knowledge_context_version, knowledge_delivery
FROM agent_task_queue
WHERE agent_id = sqlc.arg(agent_id)::uuid
  AND (issue_id = sqlc.narg(issue_id)::uuid OR chat_session_id = sqlc.narg(chat_session_id)::uuid
       OR knowledge_delivery->>'scope_id' = sqlc.arg(scope_id)::text)
  AND (session_id = sqlc.arg(native_session_id)::text OR knowledge_context_version IS NOT NULL)
ORDER BY created_at DESC, id
LIMIT 129;

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

-- A legacy/unsupported invocation cannot prove which history implicit selectors
-- would choose. Include the physical daemon/provider and owner/provider namespace,
-- not just the current task scope or runtime UUID. Immutable snapshots survive
-- runtime deletion/merge and the current-agent arm catches damaged rewritten rows.
-- name: ListTaskKnowledgeRuntimeHistory :many
SELECT task.id, task.agent_id, task.runtime_id,
       task.knowledge_context_version, task.knowledge_delivery
FROM agent_task_queue task
WHERE task.knowledge_context_version IS NOT NULL
  AND (task.runtime_id = sqlc.arg(runtime_id)::uuid
    OR task.agent_id IN (SELECT id FROM agent WHERE runtime_id = sqlc.arg(runtime_id)::uuid)
    OR task.knowledge_delivery @> jsonb_build_object('claims',jsonb_build_array(jsonb_build_object('descriptor',jsonb_build_object('runtime_id',sqlc.arg(runtime_id)::text))))
    OR task.knowledge_delivery @> jsonb_build_object('claims',jsonb_build_array(jsonb_build_object('daemon_id',sqlc.arg(daemon_id)::text,'descriptor',jsonb_build_object('provider',sqlc.arg(provider)::text))))
    OR task.knowledge_delivery @> jsonb_build_object('claims',jsonb_build_array(jsonb_build_object('runtime_owner_user_id',sqlc.arg(owner_id)::text,'descriptor',jsonb_build_object('provider',sqlc.arg(provider)::text)))))
ORDER BY task.created_at DESC, task.id
LIMIT 129;
