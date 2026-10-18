package service

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type CollaborationParticipantView struct {
	CollaborationContentAccount
	AccountActive bool `json:"account_active"`
}

type CollaborationParticipantsView struct {
	ProtocolVersion       int                            `json:"protocol_version"`
	IssueID               pgtype.UUID                    `json:"issue_id"`
	CollaborationRevision int64                          `json:"collaboration_revision,string"`
	Participants          []CollaborationParticipantView `json:"participants"`
}

// Management retains unrevoked participants even while their account is
// disabled. These rows never become mention, invocation or handoff candidates.
func (s *CollaborationService) Participants(ctx context.Context, issueID pgtype.UUID) (CollaborationParticipantsView, error) {
	out := CollaborationParticipantsView{ProtocolVersion: 1, IssueID: issueID, Participants: make([]CollaborationParticipantView, 0)}
	err := s.readCollaborationContent(ctx, issueID, func(q *db.Queries, access CollaborationIssueAccess) error {
		ids, err := q.ListCollaborationParticipantUsers(ctx, issueID)
		if err != nil {
			return err
		}
		accounts, err := collaborationContentAccounts(ctx, q, access.Issue.WorkspaceID, ids)
		if err != nil {
			return err
		}
		out.CollaborationRevision = access.Revision
		for _, id := range ids {
			account := accounts[id]
			if account == nil {
				continue
			}
			active, err := collaborationRecipientEligible(ctx, q, id, access.Issue.WorkspaceID)
			if err != nil {
				return err
			}
			out.Participants = append(out.Participants, CollaborationParticipantView{CollaborationContentAccount: *account, AccountActive: active})
		}
		return nil
	})
	return out, err
}
