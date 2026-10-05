package service

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/admission"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (s *TaskService) checkIssueExecution(ctx context.Context, q *db.Queries, issueID, triggerID pgtype.UUID, coalesced []pgtype.UUID) error {
	if err := admission.Check(ctx, q, issueID); err != nil {
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
		if !comment.DispatchEligible {
			return &admission.Blocked{IssueID: issueID}
		}
	}
	return nil
}
