package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type CollaborationDiscussionParams struct {
	IssueID   pgtype.UUID
	CommentID pgtype.UUID
	RequestID string
	Action    string
	Emoji     string
}

// Recovery contains only the original action identity and outcome. It is safe
// for its authenticated actor to read after losing access to the issue.
type CollaborationDiscussionReceipt struct {
	ProtocolVersion int         `json:"protocol_version"`
	IssueID         pgtype.UUID `json:"issue_id"`
	CommentID       pgtype.UUID `json:"comment_id"`
	RequestID       string      `json:"request_id"`
	Action          string      `json:"action"`
	Emoji           *string     `json:"emoji"`
	Status          int         `json:"status"`
	Code            string      `json:"code"`
}

func discussionReceipt(receipt CollaborationCommentReceipt) (CollaborationDiscussionReceipt, error) {
	var emoji *string
	if len(receipt.Emoji) == 0 || json.Unmarshal(receipt.Emoji, &emoji) != nil {
		return CollaborationDiscussionReceipt{}, collaborationError(http.StatusConflict, "operation_incomplete")
	}
	if receipt.Action == "resolve" || receipt.Action == "unresolve" {
		if emoji != nil {
			return CollaborationDiscussionReceipt{}, collaborationError(http.StatusConflict, "operation_incomplete")
		}
	} else if emoji == nil || !validCollaborationReactionEmoji(*emoji) {
		return CollaborationDiscussionReceipt{}, collaborationError(http.StatusConflict, "operation_incomplete")
	}
	return CollaborationDiscussionReceipt{ProtocolVersion: receipt.ProtocolVersion, IssueID: receipt.IssueID,
		CommentID: receipt.CommentID, RequestID: receipt.RequestID, Action: receipt.Action, Emoji: emoji, Status: receipt.Status, Code: receipt.Code}, nil
}

func validCollaborationReactionEmoji(value string) bool {
	return value != "" && len(value) <= 64 && utf8.ValidString(value) && strings.TrimSpace(value) == value && !strings.ContainsAny(value, "/\x00\r\n")
}

func (s *CollaborationService) ChangeDiscussion(ctx context.Context, p CollaborationDiscussionParams) (CollaborationDiscussionReceipt, error) {
	switch p.Action {
	case "resolve", "unresolve":
		if !p.CommentID.Valid || p.Emoji != "" {
			return CollaborationDiscussionReceipt{}, collaborationError(http.StatusBadRequest, "invalid_discussion_request")
		}
	case "add_reaction", "remove_reaction":
		if !validCollaborationReactionEmoji(p.Emoji) {
			return CollaborationDiscussionReceipt{}, collaborationError(http.StatusBadRequest, "invalid_discussion_request")
		}
	default:
		return CollaborationDiscussionReceipt{}, collaborationError(http.StatusBadRequest, "invalid_discussion_request")
	}
	kind := "discussion_issue_" + p.Action
	if p.CommentID.Valid {
		kind = "discussion_comment_" + p.Action
	}
	result, err := s.mutateCollaborationContent(ctx, p.IssueID, p.RequestID, kind, p, func(t *collaborationContentTx) (CollaborationCommentReceipt, error) {
		receipt := t.result("unchanged", http.StatusOK)
		receipt.CommentID, receipt.RequestID, receipt.Action = p.CommentID, p.RequestID, p.Action
		receipt.Emoji = json.RawMessage("null")
		if p.Action == "add_reaction" || p.Action == "remove_reaction" {
			var err error
			receipt.Emoji, err = json.Marshal(p.Emoji)
			if err != nil {
				return CollaborationCommentReceipt{}, err
			}
		}
		if p.CommentID.Valid {
			comment, err := t.q.GetCollaborationComment(ctx, db.GetCollaborationCommentParams{CommentID: p.CommentID, IssueID: p.IssueID, WorkspaceID: t.access.Issue.WorkspaceID})
			if errors.Is(err, pgx.ErrNoRows) {
				receipt.Status, receipt.Code = http.StatusNotFound, "not_found"
				return receipt, nil
			}
			if err != nil {
				return CollaborationCommentReceipt{}, err
			}
			if comment.DeletedAt.Valid && (p.Action == "resolve" || p.Action == "add_reaction") {
				receipt.Status, receipt.Code = http.StatusConflict, "comment_deleted"
				return receipt, nil
			}
		}
		changed, err := changeCollaborationDiscussion(ctx, t, p)
		if err != nil {
			return CollaborationCommentReceipt{}, err
		}
		if changed == 0 {
			return receipt, nil
		}
		switch p.Action {
		case "resolve":
			receipt.Code = "comment_resolved"
		case "unresolve":
			receipt.Code = "comment_unresolved"
		case "add_reaction":
			receipt.Code = "reaction_added"
		case "remove_reaction":
			receipt.Code = "reaction_removed"
		}
		receipt.EventID = util.MustParseUUID(uuid.NewString())
		receipt.EventSeq, err = t.q.AdvanceCollaborationEventSequence(ctx, p.IssueID)
		if err != nil {
			return CollaborationCommentReceipt{}, err
		}
		payload, err := json.Marshal(struct {
			CommentID pgtype.UUID `json:"comment_id"`
			Action    string      `json:"action"`
			Emoji     string      `json:"emoji,omitempty"`
		}{p.CommentID, p.Action, p.Emoji})
		if err != nil {
			return CollaborationCommentReceipt{}, err
		}
		_, err = t.q.CreateCollaborationContentEvent(ctx, db.CreateCollaborationContentEventParams{
			EventID: receipt.EventID, IssueID: p.IssueID, EventSeq: receipt.EventSeq, ActorID: t.actor, Kind: kind, Payload: payload})
		return receipt, err
	})
	if err != nil && result.Code == "" {
		return CollaborationDiscussionReceipt{}, err
	}
	receipt, decodeErr := discussionReceipt(result)
	if decodeErr != nil {
		return CollaborationDiscussionReceipt{}, decodeErr
	}
	return receipt, err
}

func changeCollaborationDiscussion(ctx context.Context, t *collaborationContentTx, p CollaborationDiscussionParams) (int64, error) {
	workspace := t.access.Issue.WorkspaceID
	switch p.Action {
	case "resolve":
		cleared, err := t.q.ClearOtherCollaborationThreadResolutions(ctx, db.ClearOtherCollaborationThreadResolutionsParams{CommentID: p.CommentID, IssueID: p.IssueID, WorkspaceID: workspace})
		if err != nil {
			return 0, err
		}
		changed, err := t.q.ResolveCollaborationComment(ctx, db.ResolveCollaborationCommentParams{CommentID: p.CommentID, IssueID: p.IssueID, WorkspaceID: workspace, ActorID: t.actor})
		return changed + cleared, err
	case "unresolve":
		return t.q.UnresolveCollaborationComment(ctx, db.UnresolveCollaborationCommentParams{CommentID: p.CommentID, IssueID: p.IssueID, WorkspaceID: workspace})
	case "add_reaction":
		if p.CommentID.Valid {
			return t.q.AddCollaborationCommentReaction(ctx, db.AddCollaborationCommentReactionParams{CommentID: p.CommentID, WorkspaceID: workspace, ActorID: t.actor, Emoji: p.Emoji})
		}
		return t.q.AddCollaborationIssueReaction(ctx, db.AddCollaborationIssueReactionParams{IssueID: p.IssueID, WorkspaceID: workspace, ActorID: t.actor, Emoji: p.Emoji})
	case "remove_reaction":
		if p.CommentID.Valid {
			return t.q.RemoveCollaborationCommentReaction(ctx, db.RemoveCollaborationCommentReactionParams{CommentID: p.CommentID, WorkspaceID: workspace, ActorID: t.actor, Emoji: p.Emoji})
		}
		return t.q.RemoveCollaborationIssueReaction(ctx, db.RemoveCollaborationIssueReactionParams{IssueID: p.IssueID, WorkspaceID: workspace, ActorID: t.actor, Emoji: p.Emoji})
	default:
		return 0, collaborationError(http.StatusBadRequest, "invalid_discussion_request")
	}
}

func (s *CollaborationService) ListIssueReactions(ctx context.Context, issueID pgtype.UUID) ([]CollaborationCommentReaction, error) {
	items := make([]CollaborationCommentReaction, 0)
	err := s.readCollaborationContent(ctx, issueID, func(q *db.Queries, _ CollaborationIssueAccess) error {
		viewer, err := collaborationNotificationViewer(ctx, q)
		if err != nil {
			return err
		}
		rows, err := q.ListIssueReactions(ctx, issueID)
		if err != nil {
			return err
		}
		grouped := make(map[string]*CollaborationCommentReaction)
		for _, row := range rows {
			entry := grouped[row.Emoji]
			if entry == nil {
				entry = &CollaborationCommentReaction{Emoji: row.Emoji}
				grouped[row.Emoji] = entry
			}
			entry.Count++
			entry.ViewerReacted = entry.ViewerReacted || (row.ActorType == "member" && row.ActorID == viewer)
		}
		for _, entry := range grouped {
			items = append(items, *entry)
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Emoji < items[j].Emoji })
		return nil
	})
	return items, err
}

func (s *CollaborationService) GetDiscussionOperation(ctx context.Context, issueID pgtype.UUID, requestID string) (CollaborationDiscussionReceipt, error) {
	if !issueID.Valid || requestID == "" || len(requestID) > 128 {
		return CollaborationDiscussionReceipt{}, collaborationError(http.StatusBadRequest, "invalid_request_id")
	}
	viewer, err := collaborationNotificationViewer(ctx, s.Queries)
	if err != nil {
		return CollaborationDiscussionReceipt{}, err
	}
	operation, err := s.Queries.GetCollaborationOperation(ctx, db.GetCollaborationOperationParams{ScopeKind: "issue", ScopeID: issueID, ActorUserID: viewer, RequestID: requestID})
	if errors.Is(err, pgx.ErrNoRows) {
		return CollaborationDiscussionReceipt{}, collaborationError(http.StatusNotFound, "operation_not_found")
	}
	if err != nil {
		return CollaborationDiscussionReceipt{}, err
	}
	if !strings.HasPrefix(operation.Kind, "discussion_comment_") && !strings.HasPrefix(operation.Kind, "discussion_issue_") {
		return CollaborationDiscussionReceipt{}, collaborationError(http.StatusNotFound, "operation_not_found")
	}
	var receipt CollaborationCommentReceipt
	if !operation.CompletedAt.Valid || json.Unmarshal(operation.Result, &receipt) != nil || receipt.IssueID != issueID ||
		receipt.RequestID != requestID || receipt.Code == "" || receipt.Code != operation.ResultCode.String ||
		receipt.ProtocolVersion != CollaborationContentProtocolVersion || receipt.Status < 200 || receipt.Status > 499 ||
		operation.Kind != "discussion_"+map[bool]string{true: "comment_", false: "issue_"}[receipt.CommentID.Valid]+receipt.Action {
		return CollaborationDiscussionReceipt{}, collaborationError(http.StatusConflict, "operation_incomplete")
	}
	return discussionReceipt(receipt)
}
