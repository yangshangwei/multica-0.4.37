-- Discussion metadata has its own event sequence. It must not advance the
-- immutable content revision used by collaboration comment editors.

-- name: ClearOtherCollaborationThreadResolutions :execrows
WITH RECURSIVE ancestors AS (
    SELECT root.id, root.parent_id FROM comment root
    WHERE root.id = @comment_id AND root.issue_id = @issue_id AND root.workspace_id = @workspace_id
    UNION
    SELECT c.id, c.parent_id FROM comment c
    JOIN ancestors a ON c.id = a.parent_id
    WHERE c.issue_id = @issue_id AND c.workspace_id = @workspace_id
), descendants AS (
    SELECT a.id FROM ancestors a WHERE a.parent_id IS NULL
    UNION
    SELECT c.id FROM comment c JOIN descendants d ON c.parent_id = d.id
    WHERE c.issue_id = @issue_id AND c.workspace_id = @workspace_id
)
UPDATE comment AS target SET resolved_at = NULL, resolved_by_type = NULL, resolved_by_id = NULL
WHERE target.id IN (SELECT d.id FROM descendants d)
  AND target.id <> @comment_id AND target.resolved_at IS NOT NULL;

-- name: ResolveCollaborationComment :execrows
UPDATE comment SET resolved_at = now(), resolved_by_type = 'member', resolved_by_id = @actor_id
WHERE id = @comment_id AND issue_id = @issue_id AND workspace_id = @workspace_id
  AND deleted_at IS NULL AND resolved_at IS NULL;

-- name: UnresolveCollaborationComment :execrows
UPDATE comment SET resolved_at = NULL, resolved_by_type = NULL, resolved_by_id = NULL
WHERE id = @comment_id AND issue_id = @issue_id AND workspace_id = @workspace_id
  AND resolved_at IS NOT NULL;

-- name: AddCollaborationCommentReaction :execrows
INSERT INTO comment_reaction (comment_id, workspace_id, actor_type, actor_id, emoji)
VALUES (@comment_id, @workspace_id, 'member', @actor_id, @emoji)
ON CONFLICT (comment_id, actor_type, actor_id, emoji) DO NOTHING;

-- name: RemoveCollaborationCommentReaction :execrows
DELETE FROM comment_reaction
WHERE comment_id = @comment_id AND workspace_id = @workspace_id
  AND actor_type = 'member' AND actor_id = @actor_id AND emoji = @emoji;

-- name: AddCollaborationIssueReaction :execrows
INSERT INTO issue_reaction (issue_id, workspace_id, actor_type, actor_id, emoji)
VALUES (@issue_id, @workspace_id, 'member', @actor_id, @emoji)
ON CONFLICT (issue_id, actor_type, actor_id, emoji) DO NOTHING;

-- name: RemoveCollaborationIssueReaction :execrows
DELETE FROM issue_reaction
WHERE issue_id = @issue_id AND workspace_id = @workspace_id
  AND actor_type = 'member' AND actor_id = @actor_id AND emoji = @emoji;
