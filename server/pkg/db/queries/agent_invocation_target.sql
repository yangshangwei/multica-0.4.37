-- Agent invocation permission targets (MUL-3963). Rows are the allow-list for
-- agents whose permission_mode = 'public_to'. See migration 130.

-- name: ListAgentInvocationTargets :many
SELECT * FROM agent_invocation_target
WHERE agent_id = $1
ORDER BY target_type ASC, created_at ASC;

-- name: ListAgentInvocationTargetsByAgentIDs :many
-- Batch load for the agent list endpoint so we don't N+1 per agent.
SELECT * FROM agent_invocation_target
WHERE agent_id = ANY(@agent_ids::uuid[])
ORDER BY agent_id, target_type ASC, created_at ASC;

-- name: CreateAgentInvocationTarget :exec
-- Idempotent upsert: re-adding an existing (agent, target_type, target)
-- refreshes created_by/created_at rather than erroring. Callers replace the
-- whole set via DeleteAgentInvocationTargets + a series of these, so the
-- ON CONFLICT is belt-and-suspenders against races.
WITH locked_agent AS MATERIALIZED (SELECT agent.id FROM agent WHERE agent.id = sqlc.arg(agent_id)::uuid FOR UPDATE)
INSERT INTO agent_invocation_target (agent_id, target_type, target_id, created_by)
SELECT id, sqlc.arg(target_type)::text, sqlc.arg(target_id)::uuid, sqlc.narg('created_by')::uuid FROM locked_agent WHERE true
ON CONFLICT (agent_id, target_type, target_id) DO UPDATE SET
    created_by = EXCLUDED.created_by,
    created_at = now();

-- name: DeleteAgentInvocationTargets :exec
-- Clears every target for an agent. Used before re-writing the allow-list so
-- the update is a wholesale replace, matching the composio_toolkit_allowlist
-- write model.
WITH locked_agent AS MATERIALIZED (SELECT agent.id FROM agent WHERE agent.id = $1 FOR UPDATE)
DELETE FROM agent_invocation_target USING locked_agent
WHERE agent_id = locked_agent.id;

-- name: DeleteAgentInvocationTargetsByMember :exec
-- Removes member-target grants pointing at a leaving user, SCOPED to a single
-- workspace. A user may belong to multiple workspaces; removing them from one
-- must NOT touch their invocation grants on agents in another workspace. Joins
-- through agent (agent_invocation_target has no workspace_id column and no FK)
-- to bound the delete to @workspace_id.
WITH locked_agents AS MATERIALIZED (
    SELECT agent.id FROM agent WHERE agent.workspace_id = @workspace_id ORDER BY agent.id FOR UPDATE
)
DELETE FROM agent_invocation_target ait
USING locked_agents a
WHERE ait.agent_id = a.id
  AND ait.target_type = 'member'
  AND ait.target_id = @target_id;

-- name: DeleteAgentInvocationTargetsBySystemRuntimeAgents :exec
-- Application-layer replacement for the (deliberately absent) agent_id ON
-- DELETE CASCADE: removes invocation targets for the system agents a runtime
-- delete is about to hard-delete. MUST run in the same tx as, and BEFORE,
-- DeleteSystemAgentsByRuntime so no orphan target rows survive the agent rows
-- they belonged to. Mirrors the agent hard-delete predicate exactly.
--
-- Scoped to kind = 'system' since MUL-5559: user agents are no longer deleted
-- with their runtime (they are unbound and keep their configuration), so
-- clearing THEIR invocation targets here would silently strip a surviving
-- agent's allow-list.
WITH locked_agents AS MATERIALIZED (
    SELECT agent.id FROM agent WHERE agent.runtime_id = $1 AND agent.kind = 'system' ORDER BY agent.id FOR UPDATE
)
DELETE FROM agent_invocation_target USING locked_agents
WHERE agent_id = locked_agents.id;
