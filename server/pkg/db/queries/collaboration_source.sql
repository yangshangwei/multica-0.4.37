-- name: ListSourceIssueCollaborations :many
SELECT i.id AS issue_id,
       creator.id AS creator_user_id, creator.name AS creator_name, creator.avatar_url AS creator_avatar_url,
       NOT EXISTS (SELECT 1 FROM member m WHERE m.workspace_id=i.workspace_id AND m.user_id=creator.id) AS creator_external,
       handler.id AS handler_user_id, handler.name AS handler_name, handler.avatar_url AS handler_avatar_url,
       NOT EXISTS (SELECT 1 FROM member m WHERE m.workspace_id=i.workspace_id AND m.user_id=handler.id) AS handler_external,
       EXISTS (
           SELECT 1 FROM issue_participant p
           JOIN "user" u ON u.id=p.user_id AND u.disabled_at IS NULL
           WHERE p.issue_id=i.id AND p.revoked_at IS NULL
             AND NOT EXISTS (SELECT 1 FROM member m WHERE m.workspace_id=i.workspace_id AND m.user_id=p.user_id)
       ) AS has_external_collaborators
FROM issue i
JOIN issue_collaboration c ON c.issue_id=i.id
LEFT JOIN "user" creator ON creator.id=i.creator_id
LEFT JOIN "user" handler ON handler.id=i.assignee_id
WHERE i.workspace_id=sqlc.arg(workspace_id) AND i.id=ANY(sqlc.arg(issue_ids)::uuid[]);

-- Only references are projected; ordinary ownership and execution stay intact.
-- name: ListCollaborationOrdinaryChildren :many
SELECT i.id, i.number, i.title
FROM issue i
WHERE i.workspace_id=sqlc.arg(workspace_id)::uuid AND i.parent_issue_id=sqlc.arg(parent_issue_id)::uuid
  AND i.admission_status IN ('not_required','accepted')
  AND NOT EXISTS (SELECT 1 FROM issue_collaboration c WHERE c.issue_id=i.id)
ORDER BY i.number ASC;

-- name: ListSourceCollaborationAccounts :many
SELECT u.id AS user_id, u.name, u.avatar_url,
       NOT EXISTS (SELECT 1 FROM member m WHERE m.workspace_id=sqlc.arg(workspace_id) AND m.user_id=u.id) AS external
FROM "user" u
WHERE u.id=ANY(sqlc.arg(user_ids)::uuid[])
  AND EXISTS (SELECT 1 FROM issue i JOIN issue_collaboration c ON c.issue_id=i.id
              WHERE i.workspace_id=sqlc.arg(workspace_id)
                AND ((i.assignee_type='member' AND i.assignee_id=u.id) OR (i.creator_type='member' AND i.creator_id=u.id)));
