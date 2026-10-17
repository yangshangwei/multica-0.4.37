package service

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/admission"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (s *TaskService) checkIssueExecution(ctx context.Context, q *db.Queries, issueID, triggerID pgtype.UUID, coalesced []pgtype.UUID) error {
	if err := admission.Check(ctx, q, issueID); err != nil {
		return err
	}
	if err := checkOrdinaryExecutionSource(ctx, q, issueID); err != nil {
		return err
	}
	ids := append(append([]pgtype.UUID{}, coalesced...), triggerID)
	for _, id := range ids {
		if !id.Valid {
			continue
		}
		comment, err := q.GetComment(ctx, id)
		if err != nil {
			return err
		}
		if comment.IssueID != issueID {
			if err := checkOrdinaryExecutionSource(ctx, q, comment.IssueID); err != nil {
				return err
			}
		}
		if !comment.DispatchEligible {
			return &admission.Blocked{IssueID: issueID}
		}
	}
	return nil
}

// Ordinary children keep their own execution contract. Only the resolved
// content source, never its parent link, determines collaboration ownership.
func checkOrdinaryExecutionSource(ctx context.Context, q *db.Queries, issueID pgtype.UUID) error {
	if !issueID.Valid {
		return nil
	}
	shared, err := q.IsCollaborationIssue(ctx, issueID)
	if err != nil {
		return collaborationError(http.StatusServiceUnavailable, "collaboration_unavailable")
	}
	if shared {
		return collaborationError(http.StatusConflict, "unsupported_collaboration_action")
	}
	return nil
}
