-- Management projections deliberately omit task titles/bodies, comments,
-- resource configuration, run identities and provider output.
-- name: ListAdminCollaborationIssues :many
SELECT i.id AS issue_id, i.number, i.workspace_id AS source_workspace_id,
       w.name AS source_name, w.issue_prefix AS source_prefix,
       i.status, COALESCE(st.category, CASE WHEN i.status IN ('backlog','todo','in_progress','in_review','blocked','done','cancelled') THEN i.status ELSE '' END)::text AS status_category,
       c.revision AS collaboration_revision, c.lifecycle_epoch,
       s.id AS session_id, s.queue_state, s.control_version,
       creator.id AS creator_user_id, creator.name AS creator_name, creator.email AS creator_email,
       COALESCE(creator.avatar_url, '')::text AS creator_avatar_url, creator.disabled_at AS creator_disabled_at,
       handler.id AS handler_user_id, handler.name AS handler_name, handler.email AS handler_email,
       COALESCE(handler.avatar_url, '')::text AS handler_avatar_url, handler.disabled_at AS handler_disabled_at,
       (SELECT count(*) FROM collaboration_run r WHERE r.session_id=s.id AND r.settled_at IS NULL)::bigint AS unsettled_runs
FROM issue i
JOIN issue_collaboration c ON c.issue_id=i.id
JOIN collaboration_session s ON s.issue_id=i.id
JOIN workspace w ON w.id=i.workspace_id
JOIN organization_workspace ow ON ow.workspace_id=w.id
JOIN organization o ON o.id=ow.organization_id AND o.internal AND o.state='active'
JOIN "user" creator ON creator.id=i.creator_id
JOIN "user" handler ON handler.id=i.assignee_id
LEFT JOIN issue_status st ON st.workspace_id=i.workspace_id AND st.key=i.status
WHERE o.id=sqlc.arg(organization_id)::uuid
  AND i.creator_type='member' AND i.assignee_type='member'
  -- Match auth.IsTemporarilyDisabledUser before LIMIT. The trim characters
  -- are the Unicode White_Space set used by Go strings.TrimSpace.
  AND (creator.disabled_at IS NOT NULL
       OR creator.id::text=ANY(sqlc.arg(temporarily_disabled_user_ids)::text[])
       OR lower(btrim(COALESCE(creator.email,''), E' \t\n\r\013\f\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000'))=ANY(sqlc.arg(temporarily_disabled_user_emails)::text[]))
  AND (sqlc.narg(after_id)::uuid IS NULL OR i.id>sqlc.narg(after_id)::uuid)
ORDER BY i.id LIMIT sqlc.arg(page_limit)::int;

-- The same scoped metadata is also used as an unlocked identity hint. Writers
-- reload it after the account/source/issue locks; recovery may read it after
-- creator restoration, without reopening authority for a new mutation.
-- name: GetAdminCollaborationIssue :one
SELECT i.id AS issue_id, i.number, i.workspace_id AS source_workspace_id,
       w.name AS source_name, w.issue_prefix AS source_prefix,
       i.status, COALESCE(st.category, CASE WHEN i.status IN ('backlog','todo','in_progress','in_review','blocked','done','cancelled') THEN i.status ELSE '' END)::text AS status_category,
       c.revision AS collaboration_revision, c.lifecycle_epoch,
       s.id AS session_id, s.queue_state, s.control_version,
       creator.id AS creator_user_id, creator.name AS creator_name, creator.email AS creator_email,
       COALESCE(creator.avatar_url, '')::text AS creator_avatar_url, creator.disabled_at AS creator_disabled_at,
       handler.id AS handler_user_id, handler.name AS handler_name, handler.email AS handler_email,
       COALESCE(handler.avatar_url, '')::text AS handler_avatar_url, handler.disabled_at AS handler_disabled_at,
       (SELECT count(*) FROM collaboration_run r WHERE r.session_id=s.id AND r.settled_at IS NULL)::bigint AS unsettled_runs
FROM issue i
JOIN issue_collaboration c ON c.issue_id=i.id
JOIN collaboration_session s ON s.issue_id=i.id
JOIN workspace w ON w.id=i.workspace_id
JOIN organization_workspace ow ON ow.workspace_id=w.id
JOIN organization o ON o.id=ow.organization_id AND o.internal AND o.state='active'
JOIN "user" creator ON creator.id=i.creator_id
JOIN "user" handler ON handler.id=i.assignee_id
LEFT JOIN issue_status st ON st.workspace_id=i.workspace_id AND st.key=i.status
WHERE o.id=sqlc.arg(organization_id)::uuid AND i.id=sqlc.arg(issue_id)::uuid
  AND i.creator_type='member' AND i.assignee_type='member';

-- name: LockAdminCollaborationOrganization :one
SELECT * FROM organization WHERE id=$1 FOR SHARE;

-- Call after the workspace fence, so workspace deletion cannot own the
-- workspace row while waiting for a mapping held by this transaction.
-- name: LockAdminCollaborationSourceScope :one
SELECT organization_id FROM organization_workspace
WHERE workspace_id=sqlc.arg(workspace_id)::uuid AND organization_id=sqlc.arg(organization_id)::uuid
FOR SHARE;

-- name: ListAdminCollaborationReopenCandidates :many
SELECT u.id AS user_id, u.name, COALESCE(u.avatar_url,'')::text AS avatar_url, u.email,
       NOT EXISTS (SELECT 1 FROM member m WHERE m.workspace_id=sqlc.arg(source_workspace_id)::uuid AND m.user_id=u.id) AS external
FROM issue_participant p
JOIN "user" u ON u.id=p.user_id
JOIN user_password_credential credential ON credential.user_id=u.id
WHERE p.issue_id=sqlc.arg(issue_id)::uuid AND p.revoked_at IS NULL
  AND u.disabled_at IS NULL AND NOT credential.must_change_password AND credential.session_version>0
  AND (sqlc.narg(after_id)::uuid IS NULL OR u.id>sqlc.narg(after_id)::uuid)
ORDER BY u.id LIMIT sqlc.arg(page_limit)::int;

-- name: CountAdminCollaborationUnsettledRuns :one
SELECT count(*)::bigint FROM collaboration_run WHERE session_id=$1 AND settled_at IS NULL;
