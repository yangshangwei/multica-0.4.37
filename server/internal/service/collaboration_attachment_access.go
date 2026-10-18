package service

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// A native link captures both read bases. Losing and reacquiring either basis
// changes the stamp, so a removed participant's old link cannot revive.
type CollaborationAttachmentReadStamp struct {
	ViewerID         pgtype.UUID `json:"viewer_id"`
	AccountEpoch     int64       `json:"account_epoch,string"`
	MemberID         pgtype.UUID `json:"member_id"`
	ParticipantEpoch int64       `json:"participant_epoch,string"`
}

func (s *CollaborationService) MintAttachmentReadStamp(ctx context.Context, issue, id pgtype.UUID) (CollaborationAttachmentReadStamp, error) {
	var stamp CollaborationAttachmentReadStamp
	row, err := s.GetAttachment(ctx, issue, pgtype.UUID{}, id)
	if err != nil {
		return stamp, err
	}
	err = s.withCollaborationAttachmentWrite(ctx, row.SourceWorkspaceID, issue, false, func(q *db.Queries, actor pgtype.UUID) error {
		// Source-only readers also need a fence before a capability can escape.
		// Security revocation takes this same user lock and advances that fence.
		if _, err := q.EnsureCollaborationAccountFence(ctx, actor); err != nil {
			return err
		}
		basis, err := q.GetCollaborationAttachmentReadBasis(ctx, db.GetCollaborationAttachmentReadBasisParams{
			SourceWorkspaceID: row.SourceWorkspaceID, ViewerID: actor, IssueID: issue})
		if err != nil {
			return err
		}
		stamp = CollaborationAttachmentReadStamp{ViewerID: actor, AccountEpoch: basis.AccountEpoch,
			MemberID: basis.MemberID, ParticipantEpoch: basis.ParticipantEpoch}
		return nil
	})
	return stamp, err
}

func (s *CollaborationService) CheckAttachmentReadStamp(ctx context.Context, issue, id pgtype.UUID, version int64, stamp CollaborationAttachmentReadStamp) (db.CollaborationAttachment, error) {
	row, err := s.GetAttachment(ctx, issue, pgtype.UUID{}, id)
	if err != nil {
		return row, err
	}
	actor, err := collaborationNotificationViewer(ctx, s.Queries)
	if err != nil {
		return row, err
	}
	if actor != stamp.ViewerID || version != row.Version || stamp.AccountEpoch < 1 {
		return row, collaborationError(http.StatusNotFound, "not_found")
	}
	basis, err := s.Queries.GetCollaborationAttachmentReadBasis(ctx, db.GetCollaborationAttachmentReadBasisParams{
		SourceWorkspaceID: row.SourceWorkspaceID, ViewerID: actor, IssueID: issue})
	if err != nil {
		return row, err
	}
	if basis.AccountEpoch != stamp.AccountEpoch || basis.MemberID != stamp.MemberID || basis.ParticipantEpoch != stamp.ParticipantEpoch {
		return row, collaborationError(http.StatusNotFound, "not_found")
	}
	return row, nil
}
