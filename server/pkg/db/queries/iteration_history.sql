-- name: ListAllIterationEvents :many
-- Domain projection always consumes the complete stream; UI page limits must
-- never change original commitment, counters, charts, or a frozen payload.
SELECT * FROM iteration_event
WHERE workspace_id = sqlc.arg('workspace_id') AND iteration_id = sqlc.arg('iteration_id')
ORDER BY sequence;

-- name: ListIterationHistoryIssueReferences :many
-- Nullable references remain nullable. Every child reference is tenant scoped,
-- including the member-to-user join; archived custom statuses remain readable.
SELECT i.id AS issue_id, w.issue_prefix, w.name AS workspace_name, p.title AS project_name,
       u.name AS member_name, a.name AS agent_name, s.name AS squad_name,
       a.archived_at AS agent_archived_at, s.archived_at AS squad_archived_at,
       leader.id AS squad_leader_id, leader.archived_at AS squad_leader_archived_at,
       status.category AS status_category
FROM issue i
JOIN workspace w ON w.id = i.workspace_id
LEFT JOIN project p ON p.id = i.project_id AND p.workspace_id = i.workspace_id
LEFT JOIN member m ON i.assignee_type = 'member' AND m.user_id = i.assignee_id
    AND m.workspace_id = i.workspace_id
LEFT JOIN "user" u ON u.id = m.user_id
LEFT JOIN agent a ON i.assignee_type = 'agent' AND a.id = i.assignee_id
    AND a.workspace_id = i.workspace_id
LEFT JOIN squad s ON i.assignee_type = 'squad' AND s.id = i.assignee_id
    AND s.workspace_id = i.workspace_id
LEFT JOIN agent leader ON leader.id = s.leader_id AND leader.workspace_id = i.workspace_id
LEFT JOIN issue_status status ON status.workspace_id = i.workspace_id AND status.key = i.status
WHERE i.workspace_id = sqlc.arg('workspace_id') AND i.id = ANY(sqlc.arg('issue_ids')::uuid[])
ORDER BY i.id;

-- name: LockIterationHistoryWorkspace :one
-- KEY SHARE alone does not protect issue_prefix from an ordinary rename.
SELECT issue_prefix FROM workspace WHERE id = $1 FOR SHARE NOWAIT;

-- name: LockIterationHistoryMembers :many
-- Freeze both membership and the globally stored user display name. NOWAIT
-- avoids reversing an unrelated user's member/profile write lock order.
SELECT m.user_id, u.name
FROM member m JOIN "user" u ON u.id = m.user_id
WHERE m.workspace_id = sqlc.arg('workspace_id') AND m.user_id = ANY(sqlc.arg('user_ids')::uuid[])
ORDER BY m.user_id FOR SHARE OF m, u NOWAIT;
