package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (s *CollaborationService) EditComment(ctx context.Context, p CollaborationEditCommentParams) (CollaborationCommentReceipt, error) {
	return s.changeCollaborationComment(ctx, p, false)
}

func (s *CollaborationService) DeleteComment(ctx context.Context, p CollaborationEditCommentParams) (CollaborationCommentReceipt, error) {
	return s.changeCollaborationComment(ctx, p, true)
}

func (s *CollaborationService) changeCollaborationComment(ctx context.Context, p CollaborationEditCommentParams, deleting bool) (CollaborationCommentReceipt, error) {
	if !p.CommentID.Valid || p.ExpectedCommentRevision < 1 || (!deleting && (strings.TrimSpace(util.SanitizeTextForPostgres(p.Content)) == "" || len(p.Content) > 64<<10)) {
		return CollaborationCommentReceipt{}, collaborationError(http.StatusBadRequest, "invalid_comment_request")
	}
	if deleting && p.Content != "" {
		return CollaborationCommentReceipt{}, collaborationError(http.StatusBadRequest, "invalid_comment_request")
	}
	kind := "edit_comment"
	if deleting {
		kind = "delete_comment"
	}
	return s.mutateCollaborationContent(ctx, p.IssueID, p.RequestID, kind, p, func(t *collaborationContentTx) (CollaborationCommentReceipt, error) {
		before, err := t.q.GetCollaborationComment(ctx, db.GetCollaborationCommentParams{CommentID: p.CommentID, IssueID: p.IssueID, WorkspaceID: t.access.Issue.WorkspaceID})
		if errors.Is(err, pgx.ErrNoRows) {
			return t.result("not_found", http.StatusNotFound), nil
		}
		if err != nil {
			return CollaborationCommentReceipt{}, err
		}
		if before.AuthorType != "member" || before.AuthorID != t.actor || before.Type != "comment" {
			return t.result("forbidden", http.StatusForbidden), nil
		}
		receipt := t.result("stale_comment_revision", http.StatusConflict)
		receipt.CommentID, receipt.CommentRevision = before.ID, before.Revision
		if before.DeletedAt.Valid {
			receipt.Code = "comment_deleted"
			return receipt, nil
		}
		if before.Revision != p.ExpectedCommentRevision {
			return receipt, nil
		}
		content := util.SanitizeTextForPostgres(p.Content)
		if !deleting && content == before.Content {
			receipt.Status, receipt.Code = http.StatusOK, "unchanged"
			return receipt, nil
		}
		var after db.Comment
		if deleting {
			after, err = t.q.DeleteCollaborationComment(ctx, db.DeleteCollaborationCommentParams{
				CommentID: p.CommentID, IssueID: p.IssueID, WorkspaceID: t.access.Issue.WorkspaceID, AuthorID: t.actor, ExpectedRevision: p.ExpectedCommentRevision})
		} else {
			after, err = t.q.EditCollaborationComment(ctx, db.EditCollaborationCommentParams{
				CommentID: p.CommentID, IssueID: p.IssueID, WorkspaceID: t.access.Issue.WorkspaceID, AuthorID: t.actor,
				ExpectedRevision: p.ExpectedCommentRevision, Content: content})
		}
		if err != nil {
			return CollaborationCommentReceipt{}, err
		}
		receipt.Status, receipt.Code, receipt.CommentRevision = http.StatusOK, "comment_edited", after.Revision
		if deleting {
			receipt.Code = "comment_deleted"
		}
		if err = t.recordCommentEvent(ctx, &receipt, &before.Content, after.Content); err != nil {
			return CollaborationCommentReceipt{}, err
		}
		if !deleting {
			err = notifyAddedCollaborationMentions(ctx, t.q, t.access, t.actor, before.ID, receipt.EventSeq, "comment", before.Content, after.Content)
		}
		return receipt, err
	})
}

func (t *collaborationContentTx) recordCommentEvent(ctx context.Context, receipt *CollaborationCommentReceipt, before *string, after string) error {
	var err error
	if !receipt.EventID.Valid {
		receipt.EventID = util.MustParseUUID(uuid.NewString())
	}
	receipt.EventSeq, err = t.q.AdvanceCollaborationEventSequence(ctx, t.access.Issue.ID)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(collaborationCommentHistoryPayload{
		CommentID: receipt.CommentID, CommentRevision: receipt.CommentRevision, Before: before, After: after, RequestID: t.operation.RequestID})
	if err != nil {
		return err
	}
	_, err = t.q.CreateCollaborationContentEvent(ctx, db.CreateCollaborationContentEventParams{
		EventID: receipt.EventID, IssueID: t.access.Issue.ID, EventSeq: receipt.EventSeq, ActorID: t.actor, Kind: receipt.Code, Payload: payload})
	return err
}

// GetCommentOperation recovers the original content-free result after an
// uncertain response. Unlike content reads, the actor may have since departed.
func (s *CollaborationService) GetCommentOperation(ctx context.Context, issueID pgtype.UUID, requestID string) (CollaborationCommentReceipt, error) {
	if requestID == "" || len(requestID) > 128 {
		return CollaborationCommentReceipt{}, collaborationError(http.StatusBadRequest, "invalid_request_id")
	}
	viewer, err := collaborationNotificationViewer(ctx, s.Queries)
	if err != nil {
		return CollaborationCommentReceipt{}, err
	}
	operation, err := s.Queries.GetCollaborationOperation(ctx, db.GetCollaborationOperationParams{
		ScopeKind: "issue", ScopeID: issueID, ActorUserID: viewer, RequestID: requestID})
	if errors.Is(err, pgx.ErrNoRows) {
		return CollaborationCommentReceipt{}, collaborationError(http.StatusNotFound, "not_found")
	}
	if err != nil {
		return CollaborationCommentReceipt{}, err
	}
	if operation.Kind != "create_comment" && operation.Kind != "edit_comment" && operation.Kind != "delete_comment" {
		return CollaborationCommentReceipt{}, collaborationError(http.StatusNotFound, "not_found")
	}
	var receipt CollaborationCommentReceipt
	if !operation.CompletedAt.Valid || json.Unmarshal(operation.Result, &receipt) != nil || receipt.IssueID != issueID || receipt.Code == "" || receipt.Code != operation.ResultCode.String {
		return CollaborationCommentReceipt{}, collaborationError(http.StatusConflict, "operation_incomplete")
	}
	if _, err = s.ResolveIssueAccess(ctx, s.Queries, issueID); err != nil {
		var domain *CollaborationError
		if !errors.As(err, &domain) || domain.Code != "not_found" {
			return CollaborationCommentReceipt{}, err
		}
		// Only completion identity survives loss of access; former recipients
		// cannot use recovery to enumerate targets or current commands.
		receipt.Targets = make([]CollaborationCommentTargetResult, 0)
	}
	return receipt, nil
}
