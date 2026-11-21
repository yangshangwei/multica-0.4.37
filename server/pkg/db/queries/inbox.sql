-- name: ListInboxItems :many
SELECT i.*,
       iss.status AS issue_status,
       iss.priority AS issue_priority,
       CASE WHEN NOT i.archived AND s.snoozed_until<=now() THEN false ELSE i.read END::boolean AS effective_read,
       CASE WHEN i.archived THEN NULL::timestamptz ELSE s.snoozed_until END::timestamptz AS snoozed_until,
       COALESCE(s.revision,0)::bigint AS snooze_revision,
       CASE WHEN NOT i.archived AND s.snoozed_until<=now() THEN s.snoozed_until ELSE s.reminded_at END::timestamptz AS reminded_at,
       COALESCE(CASE WHEN NOT i.archived AND s.snoozed_until<=now() THEN 'due'::text ELSE s.reminder_reason END,'')::text AS reminder_reason
FROM inbox_item i
LEFT JOIN issue iss ON iss.id = i.issue_id
LEFT JOIN inbox_snooze s ON s.workspace_id=i.workspace_id
  AND s.recipient_type=i.recipient_type AND s.recipient_id=i.recipient_id
  AND s.group_kind=CASE WHEN i.issue_id IS NULL THEN 'item' ELSE 'issue' END
  AND s.group_id=COALESCE(i.issue_id,i.id)
WHERE i.workspace_id = $1 AND i.recipient_type = $2 AND i.recipient_id = $3 AND i.archived = false
  AND (s.snoozed_until IS NULL OR s.snoozed_until<=now())
  AND (i.issue_id IS NULL OR iss.id IS NOT NULL)
ORDER BY GREATEST(COALESCE(s.reminded_at,s.snoozed_until,i.created_at),i.created_at) DESC, i.created_at DESC, i.id DESC;

-- name: ListArchivedInboxItems :many
-- Archived counterpart of ListInboxItems, backing the inbox's "Archived"
-- sub-view (MUL-3736).
--
-- An issue whose group still has an active row is excluded: archiving is
-- issue-level, so a NEW notification on an already-archived issue leaves the
-- old archived rows in place alongside the fresh active one. The issue belongs
-- in the main inbox at that point, and the two lists must stay mutually
-- exclusive per issue group — otherwise the same issue renders in both. The
-- exclusion lives here rather than in the client so neither list depends on
-- the other's cache being loaded. Items without an issue_id group on their own
-- id and can never have an active sibling, hence the IS NULL short-circuit.
--
-- LIMIT applies to ISSUE GROUPS, not raw notification rows. Applying it after
-- the final SELECT lets one noisy issue consume all 200 rows and hide other
-- archived issues, which makes both the visible list and its count too small.
-- The response stays bounded at two rows per group: the newest row the UI
-- renders plus, when different, the newest row carrying a comment anchor. The
-- client already merges those two rows while deduplicating, preserving direct
-- comment landing without sending every historical notification in the group.
-- Keep the materialized working set narrow: full row data is joined only for
-- the final selected ids, not copied for every archived notification scanned.
WITH eligible_archived AS MATERIALIZED (
    SELECT i.id,
           COALESCE(i.issue_id, i.id) AS group_id,
           i.created_at,
           i.details
    FROM inbox_item i
    WHERE i.workspace_id = $1
      AND i.recipient_type = $2
      AND i.recipient_id = $3
      AND i.archived = true
      AND (i.issue_id IS NULL OR NOT EXISTS (
          SELECT 1
          FROM inbox_item active
          WHERE active.workspace_id = i.workspace_id
            AND active.recipient_type = i.recipient_type
            AND active.recipient_id = i.recipient_id
            AND active.issue_id = i.issue_id
            AND active.archived = false
      ))
), newest_groups AS (
    SELECT DISTINCT ON (group_id)
           group_id,
           id AS newest_id,
           created_at AS newest_created_at
    FROM eligible_archived
    ORDER BY group_id, created_at DESC, id DESC
), limited_groups AS (
    SELECT group_id, newest_id
    FROM newest_groups
    ORDER BY newest_created_at DESC, newest_id DESC
    LIMIT 200
), comment_anchors AS (
    SELECT DISTINCT ON (archived.group_id)
           archived.group_id,
           archived.id
    FROM eligible_archived archived
    JOIN limited_groups selected USING (group_id)
    WHERE NULLIF(archived.details->>'comment_id', '') IS NOT NULL
    ORDER BY archived.group_id, archived.created_at DESC, archived.id DESC
), selected_ids AS (
    SELECT newest_id AS id FROM limited_groups
    UNION
    SELECT id FROM comment_anchors
)
SELECT i.*,
       iss.status AS issue_status,
       iss.priority AS issue_priority,
       CASE WHEN NOT i.archived AND s.snoozed_until<=now() THEN false ELSE i.read END::boolean AS effective_read,
       CASE WHEN i.archived THEN NULL::timestamptz ELSE s.snoozed_until END::timestamptz AS snoozed_until,
       COALESCE(s.revision,0)::bigint AS snooze_revision,
       CASE WHEN NOT i.archived AND s.snoozed_until<=now() THEN s.snoozed_until ELSE s.reminded_at END::timestamptz AS reminded_at,
       COALESCE(CASE WHEN NOT i.archived AND s.snoozed_until<=now() THEN 'due'::text ELSE s.reminder_reason END,'')::text AS reminder_reason
FROM inbox_item i
JOIN selected_ids selected ON selected.id = i.id
LEFT JOIN issue iss ON iss.id = i.issue_id
LEFT JOIN inbox_snooze s ON s.workspace_id=i.workspace_id
  AND s.recipient_type=i.recipient_type AND s.recipient_id=i.recipient_id
  AND s.group_kind=CASE WHEN i.issue_id IS NULL THEN 'item' ELSE 'issue' END
  AND s.group_id=COALESCE(i.issue_id,i.id)
ORDER BY i.created_at DESC, i.id DESC;

-- name: ListSnoozedInboxItems :many
SELECT i.*,
       iss.status AS issue_status,
       iss.priority AS issue_priority,
       CASE WHEN NOT i.archived AND s.snoozed_until<=now() THEN false ELSE i.read END::boolean AS effective_read,
       CASE WHEN i.archived THEN NULL::timestamptz ELSE s.snoozed_until END::timestamptz AS snoozed_until,
       COALESCE(s.revision,0)::bigint AS snooze_revision,
       CASE WHEN NOT i.archived AND s.snoozed_until<=now() THEN s.snoozed_until ELSE s.reminded_at END::timestamptz AS reminded_at,
       COALESCE(CASE WHEN NOT i.archived AND s.snoozed_until<=now() THEN 'due'::text ELSE s.reminder_reason END,'')::text AS reminder_reason
FROM inbox_item i
LEFT JOIN issue iss ON iss.id=i.issue_id
LEFT JOIN inbox_snooze s ON s.workspace_id=i.workspace_id
  AND s.recipient_type=i.recipient_type AND s.recipient_id=i.recipient_id
  AND s.group_kind=CASE WHEN i.issue_id IS NULL THEN 'item' ELSE 'issue' END
  AND s.group_id=COALESCE(i.issue_id,i.id)
WHERE i.workspace_id=$1 AND i.recipient_type=$2 AND i.recipient_id=$3 AND i.archived=false
  AND s.snoozed_until>now() AND (i.issue_id IS NULL OR iss.id IS NOT NULL)
ORDER BY s.snoozed_until, i.created_at DESC, i.id DESC;

-- name: GetInboxItem :one
SELECT * FROM inbox_item
WHERE id = $1;

-- name: GetInboxItemInWorkspace :one
SELECT * FROM inbox_item
WHERE id = $1 AND workspace_id = $2;

-- name: CreateInboxItem :one
INSERT INTO inbox_item (
    workspace_id, recipient_type, recipient_id,
    type, severity, issue_id, title, body,
    actor_type, actor_id, details, id
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, COALESCE(sqlc.narg('id')::uuid, gen_random_uuid()))
RETURNING *;

-- name: CreateCommentInboxItem :one
-- Preserve every retained delivery of this original comment, including legacy
-- random IDs and read/archived rows. The stable ID closes concurrent replays by
-- current writers; this does not fence an older random-ID writer or remove old
-- duplicates. Compare canonical UUID text without casting historical JSON.
INSERT INTO inbox_item (
    id, workspace_id, recipient_type, recipient_id,
    type, severity, issue_id, title, body, actor_type, actor_id, details
)
SELECT sqlc.arg('id')::uuid, sqlc.arg('workspace_id')::uuid, 'member', sqlc.arg('recipient_id')::uuid,
       'new_comment', sqlc.arg('severity')::text, sqlc.arg('issue_id')::uuid, sqlc.arg('title')::text,
       sqlc.narg('body')::text, sqlc.narg('actor_type')::text, sqlc.narg('actor_id')::uuid, sqlc.arg('details')::jsonb
WHERE NOT EXISTS (
    SELECT 1 FROM inbox_item retained
    WHERE retained.workspace_id = sqlc.arg('workspace_id')::uuid
      AND retained.issue_id = sqlc.arg('issue_id')::uuid
      AND retained.recipient_type = 'member'
      AND retained.recipient_id = sqlc.arg('recipient_id')::uuid
      AND retained.type = 'new_comment'
      AND lower(retained.details->>'comment_id') = sqlc.arg('comment_id')::uuid::text
)
ON CONFLICT (id) DO NOTHING
RETURNING *;

-- name: MarkInboxRead :one
UPDATE inbox_item SET read = true
WHERE id = $1
RETURNING *;

-- name: MarkInboxUnread :one
-- Exact inverse of MarkInboxRead, and item-level for the same reason it is:
-- the inbox renders one row per issue carrying that group's NEWEST item, and
-- the group's read state is that item's read state. Flipping the whole group
-- unread would overwrite older siblings' retained read decisions without
-- changing what the grouped UI or unread count shows.
UPDATE inbox_item SET read = false
WHERE id = $1
RETURNING *;

-- name: ArchiveInboxItem :one
UPDATE inbox_item SET archived = true
WHERE id = $1
RETURNING *;

-- name: ArchiveInboxByIssue :execrows
UPDATE inbox_item SET archived = true
WHERE workspace_id = $1 AND recipient_type = $2 AND recipient_id = $3 AND issue_id = $4 AND archived = false;

-- name: UnarchiveInboxItem :one
-- Deliberately does not touch `read`: unarchiving restores an item to the main
-- inbox in the exact read/unread state it was archived in, so restoring an
-- unread item legitimately raises the unread badge again (MUL-3736).
UPDATE inbox_item SET archived = false
WHERE id = $1
RETURNING *;

-- name: UnarchiveInboxByIssue :execrows
-- Issue-level restore, mirroring ArchiveInboxByIssue: archiving one item
-- archives every sibling for the same issue, so unarchiving must bring the
-- whole group back. Leaves `read` untouched for the same reason as above.
UPDATE inbox_item SET archived = false
WHERE workspace_id = $1 AND recipient_type = $2 AND recipient_id = $3 AND issue_id = $4 AND archived = true;

-- name: ArchiveInboxByIssueAndType :many
UPDATE inbox_item SET archived = true
WHERE workspace_id = $1 AND issue_id = $2 AND type = $3 AND archived = false
RETURNING recipient_type, recipient_id;

-- name: CountUnreadInbox :one
SELECT count(*) FROM (
 SELECT DISTINCT ON (CASE WHEN i.issue_id IS NULL THEN 'item' ELSE 'issue' END,COALESCE(i.issue_id,i.id))
  CASE WHEN s.snoozed_until<=now() THEN false ELSE i.read END AS effective_read
 FROM inbox_item i
 LEFT JOIN inbox_snooze s ON s.workspace_id=i.workspace_id AND s.recipient_type=i.recipient_type AND s.recipient_id=i.recipient_id
 AND s.group_kind=CASE WHEN i.issue_id IS NULL THEN 'item' ELSE 'issue' END AND s.group_id=COALESCE(i.issue_id,i.id)
 LEFT JOIN issue iss ON iss.id=i.issue_id AND iss.workspace_id=i.workspace_id
 WHERE i.workspace_id=$1 AND i.recipient_type=$2 AND i.recipient_id=$3 AND i.archived=false
 AND (s.snoozed_until IS NULL OR s.snoozed_until<=now()) AND (i.issue_id IS NULL OR iss.id IS NOT NULL)
 ORDER BY CASE WHEN i.issue_id IS NULL THEN 'item' ELSE 'issue' END,COALESCE(i.issue_id,i.id),i.created_at DESC,i.id DESC
) newest WHERE newest.effective_read=false;

-- name: CountUnreadInboxByWorkspace :many
-- Count the rendered newest group with the same effective overdue state as lists.
SELECT newest.workspace_id,count(*) AS count FROM (
 SELECT DISTINCT ON (i.workspace_id,CASE WHEN i.issue_id IS NULL THEN 'item' ELSE 'issue' END,COALESCE(i.issue_id,i.id))
  i.workspace_id,CASE WHEN s.snoozed_until<=now() THEN false ELSE i.read END AS effective_read
 FROM inbox_item i
 JOIN member m ON m.workspace_id=i.workspace_id AND m.user_id=i.recipient_id
 LEFT JOIN inbox_snooze s ON s.workspace_id=i.workspace_id AND s.recipient_type=i.recipient_type AND s.recipient_id=i.recipient_id
 AND s.group_kind=CASE WHEN i.issue_id IS NULL THEN 'item' ELSE 'issue' END AND s.group_id=COALESCE(i.issue_id,i.id)
 LEFT JOIN issue iss ON iss.id=i.issue_id AND iss.workspace_id=i.workspace_id
 WHERE i.recipient_type='member' AND i.recipient_id=$1 AND i.archived=false
 AND (s.snoozed_until IS NULL OR s.snoozed_until<=now()) AND (i.issue_id IS NULL OR iss.id IS NOT NULL)
 ORDER BY i.workspace_id,CASE WHEN i.issue_id IS NULL THEN 'item' ELSE 'issue' END,COALESCE(i.issue_id,i.id),i.created_at DESC,i.id DESC
) newest WHERE newest.effective_read=false GROUP BY newest.workspace_id;

-- name: MarkAllInboxRead :execrows
WITH active_groups AS (SELECT DISTINCT ON (CASE WHEN i.issue_id IS NULL THEN 'item' ELSE 'issue' END,COALESCE(i.issue_id,i.id))
 CASE WHEN i.issue_id IS NULL THEN 'item' ELSE 'issue' END AS group_kind,COALESCE(i.issue_id,i.id) AS group_id,
 CASE WHEN s.snoozed_until<=now() THEN false ELSE i.read END AS effective_read,
 (s.snoozed_until IS NOT NULL AND s.snoozed_until<=now()) AS overdue
 FROM inbox_item i LEFT JOIN inbox_snooze s ON s.workspace_id=i.workspace_id AND s.recipient_type=i.recipient_type AND s.recipient_id=i.recipient_id
 AND s.group_kind=CASE WHEN i.issue_id IS NULL THEN 'item' ELSE 'issue' END AND s.group_id=COALESCE(i.issue_id,i.id)
 WHERE i.workspace_id=$1 AND i.recipient_type='member' AND i.recipient_id=$2 AND i.archived=false AND (s.snoozed_until IS NULL OR s.snoozed_until<=now())
 ORDER BY CASE WHEN i.issue_id IS NULL THEN 'item' ELSE 'issue' END,COALESCE(i.issue_id,i.id),i.created_at DESC,i.id DESC), selected_groups AS (
 SELECT * FROM active_groups WHERE effective_read=false
), changed_state AS (UPDATE inbox_snooze s SET revision=s.revision+1,reminded_at=CASE WHEN s.snoozed_until<=now() THEN s.snoozed_until ELSE s.reminded_at END,reminder_reason=CASE WHEN s.snoozed_until<=now() THEN 'due' ELSE s.reminder_reason END,snoozed_until=NULL,undo_token=NULL,undo_snapshot=NULL
 FROM selected_groups g WHERE s.workspace_id=$1 AND s.recipient_type='member' AND s.recipient_id=$2 AND s.group_kind=g.group_kind AND s.group_id=g.group_id)
UPDATE inbox_item i SET read=true FROM selected_groups g
WHERE i.workspace_id=$1 AND i.recipient_type='member' AND i.recipient_id=$2 AND i.archived=false
 AND CASE WHEN i.issue_id IS NULL THEN 'item' ELSE 'issue' END=g.group_kind AND COALESCE(i.issue_id,i.id)=g.group_id
 AND (i.read=false OR g.overdue);

-- name: ArchiveAllInbox :execrows
WITH selected_groups AS (SELECT DISTINCT ON (CASE WHEN i.issue_id IS NULL THEN 'item' ELSE 'issue' END,COALESCE(i.issue_id,i.id))
 CASE WHEN i.issue_id IS NULL THEN 'item' ELSE 'issue' END AS group_kind,COALESCE(i.issue_id,i.id) AS group_id,
 CASE WHEN s.snoozed_until<=now() THEN false ELSE i.read END AS effective_read,
 (s.snoozed_until IS NOT NULL AND s.snoozed_until<=now()) AS overdue
 FROM inbox_item i LEFT JOIN inbox_snooze s ON s.workspace_id=i.workspace_id AND s.recipient_type=i.recipient_type AND s.recipient_id=i.recipient_id
 AND s.group_kind=CASE WHEN i.issue_id IS NULL THEN 'item' ELSE 'issue' END AND s.group_id=COALESCE(i.issue_id,i.id)
 WHERE i.workspace_id=$1 AND i.recipient_type='member' AND i.recipient_id=$2 AND i.archived=false AND (s.snoozed_until IS NULL OR s.snoozed_until<=now())
 ORDER BY CASE WHEN i.issue_id IS NULL THEN 'item' ELSE 'issue' END,COALESCE(i.issue_id,i.id),i.created_at DESC,i.id DESC), changed_state AS (UPDATE inbox_snooze s SET revision=s.revision+1,snoozed_until=NULL,undo_token=NULL,undo_snapshot=NULL,reminded_at=NULL,reminder_reason=NULL
 FROM selected_groups g WHERE s.workspace_id=$1 AND s.recipient_type='member' AND s.recipient_id=$2 AND s.group_kind=g.group_kind AND s.group_id=g.group_id)
UPDATE inbox_item i SET archived=true FROM selected_groups g
WHERE i.workspace_id=$1 AND i.recipient_type='member' AND i.recipient_id=$2 AND i.archived=false
 AND CASE WHEN i.issue_id IS NULL THEN 'item' ELSE 'issue' END=g.group_kind AND COALESCE(i.issue_id,i.id)=g.group_id;

-- name: ArchiveAllReadInbox :execrows
-- An unmaterialized expired read row is effectively unread and must stay visible.
WITH active_groups AS (SELECT DISTINCT ON (CASE WHEN i.issue_id IS NULL THEN 'item' ELSE 'issue' END,COALESCE(i.issue_id,i.id))
 CASE WHEN i.issue_id IS NULL THEN 'item' ELSE 'issue' END AS group_kind,COALESCE(i.issue_id,i.id) AS group_id,
 CASE WHEN s.snoozed_until<=now() THEN false ELSE i.read END AS effective_read,
 (s.snoozed_until IS NOT NULL AND s.snoozed_until<=now()) AS overdue
 FROM inbox_item i LEFT JOIN inbox_snooze s ON s.workspace_id=i.workspace_id AND s.recipient_type=i.recipient_type AND s.recipient_id=i.recipient_id
 AND s.group_kind=CASE WHEN i.issue_id IS NULL THEN 'item' ELSE 'issue' END AND s.group_id=COALESCE(i.issue_id,i.id)
 WHERE i.workspace_id=$1 AND i.recipient_type='member' AND i.recipient_id=$2 AND i.archived=false AND (s.snoozed_until IS NULL OR s.snoozed_until<=now())
 ORDER BY CASE WHEN i.issue_id IS NULL THEN 'item' ELSE 'issue' END,COALESCE(i.issue_id,i.id),i.created_at DESC,i.id DESC),selected_groups AS (
 SELECT * FROM active_groups WHERE effective_read=true
),changed_state AS (UPDATE inbox_snooze s SET revision=s.revision+1,snoozed_until=NULL,undo_token=NULL,undo_snapshot=NULL,reminded_at=NULL,reminder_reason=NULL
 FROM selected_groups g WHERE s.workspace_id=$1 AND s.recipient_type='member' AND s.recipient_id=$2 AND s.group_kind=g.group_kind AND s.group_id=g.group_id)
UPDATE inbox_item i SET archived=true FROM selected_groups g
WHERE i.workspace_id=$1 AND i.recipient_type='member' AND i.recipient_id=$2 AND i.archived=false
 AND CASE WHEN i.issue_id IS NULL THEN 'item' ELSE 'issue' END=g.group_kind AND COALESCE(i.issue_id,i.id)=g.group_id;

-- name: ArchiveCompletedInbox :execrows
WITH selected_groups AS (SELECT DISTINCT ON (CASE WHEN i.issue_id IS NULL THEN 'item' ELSE 'issue' END,COALESCE(i.issue_id,i.id))
 CASE WHEN i.issue_id IS NULL THEN 'item' ELSE 'issue' END AS group_kind,COALESCE(i.issue_id,i.id) AS group_id,
 CASE WHEN s.snoozed_until<=now() THEN false ELSE i.read END AS effective_read,
 (s.snoozed_until IS NOT NULL AND s.snoozed_until<=now()) AS overdue
 FROM inbox_item i LEFT JOIN inbox_snooze s ON s.workspace_id=i.workspace_id AND s.recipient_type=i.recipient_type AND s.recipient_id=i.recipient_id
 AND s.group_kind=CASE WHEN i.issue_id IS NULL THEN 'item' ELSE 'issue' END AND s.group_id=COALESCE(i.issue_id,i.id)
 WHERE i.workspace_id=$1 AND i.recipient_type='member' AND i.recipient_id=$2 AND i.archived=false AND i.issue_id IN (SELECT iss.id FROM issue iss WHERE iss.workspace_id=$1 AND iss.status=ANY(sqlc.arg('terminal_status_keys')::text[])) AND (s.snoozed_until IS NULL OR s.snoozed_until<=now())
 ORDER BY CASE WHEN i.issue_id IS NULL THEN 'item' ELSE 'issue' END,COALESCE(i.issue_id,i.id),i.created_at DESC,i.id DESC),changed_state AS (UPDATE inbox_snooze s SET revision=s.revision+1,snoozed_until=NULL,undo_token=NULL,undo_snapshot=NULL,reminded_at=NULL,reminder_reason=NULL
 FROM selected_groups g WHERE s.workspace_id=$1 AND s.recipient_type='member' AND s.recipient_id=$2 AND s.group_kind=g.group_kind AND s.group_id=g.group_id)
UPDATE inbox_item i SET archived=true FROM selected_groups g
WHERE i.workspace_id=$1 AND i.recipient_type='member' AND i.recipient_id=$2 AND i.archived=false
 AND CASE WHEN i.issue_id IS NULL THEN 'item' ELSE 'issue' END=g.group_kind AND COALESCE(i.issue_id,i.id)=g.group_id;
