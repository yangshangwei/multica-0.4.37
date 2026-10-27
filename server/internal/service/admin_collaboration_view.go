package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type AdminCollaborationAccount struct {
	UserID    pgtype.UUID `json:"user_id"`
	Name      string      `json:"name"`
	AvatarURL string      `json:"avatar_url"`
	Disabled  bool        `json:"disabled"`
}

type AdminCollaborationIssue struct {
	IssueID               pgtype.UUID               `json:"issue_id"`
	Identifier            string                    `json:"identifier"`
	SourceWorkspaceID     pgtype.UUID               `json:"source_workspace_id"`
	SourceName            string                    `json:"source_name"`
	Status                string                    `json:"status"`
	StatusCategory        string                    `json:"status_category"`
	CollaborationRevision int64                     `json:"collaboration_revision,string"`
	Creator               AdminCollaborationAccount `json:"creator"`
	Handler               AdminCollaborationAccount `json:"handler"`
	QueueState            string                    `json:"queue_state"`
	UnsettledRuns         int64                     `json:"unsettled_runs"`
}

type AdminCollaborationDetail struct {
	AdminCollaborationIssue
	Scope             pgtype.UUID                  `json:"scope"`
	Actions           []CollaborationStatusOptions `json:"actions"`
	SuccessorRequired bool                         `json:"successor_required"`
}

type AdminCollaborationPageQuery struct {
	Limit  int
	Cursor pgtype.UUID
}

type AdminCollaborationPage struct {
	Scope      pgtype.UUID               `json:"scope"`
	Items      []AdminCollaborationIssue `json:"items"`
	NextCursor pgtype.UUID               `json:"next_cursor"`
}

type AdminCollaborationCandidatePage struct {
	Scope                 pgtype.UUID                   `json:"scope"`
	IssueID               pgtype.UUID                   `json:"issue_id"`
	CollaborationRevision int64                         `json:"collaboration_revision,string"`
	Items                 []CollaborationContentAccount `json:"items"`
	NextCursor            pgtype.UUID                   `json:"next_cursor"`
}

func adminCollaborationIssueView(row db.GetAdminCollaborationIssueRow) AdminCollaborationIssue {
	return AdminCollaborationIssue{IssueID: row.IssueID, Identifier: fmt.Sprintf("%s-%d", row.SourcePrefix, row.Number),
		SourceWorkspaceID: row.SourceWorkspaceID, SourceName: row.SourceName, Status: row.Status, StatusCategory: row.StatusCategory,
		CollaborationRevision: row.CollaborationRevision, QueueState: row.QueueState, UnsettledRuns: row.UnsettledRuns,
		Creator: AdminCollaborationAccount{UserID: row.CreatorUserID, Name: row.CreatorName, AvatarURL: row.CreatorAvatarUrl,
			Disabled: adminCollaborationAccountDisabled(row.CreatorUserID, row.CreatorEmail, row.CreatorDisabledAt)},
		Handler: AdminCollaborationAccount{UserID: row.HandlerUserID, Name: row.HandlerName, AvatarURL: row.HandlerAvatarUrl,
			Disabled: adminCollaborationAccountDisabled(row.HandlerUserID, row.HandlerEmail, row.HandlerDisabledAt)}}
}

func adminCollaborationAccountDisabled(id pgtype.UUID, email pgtype.Text, disabledAt pgtype.Timestamptz) bool {
	return disabledAt.Valid || auth.IsTemporarilyDisabledUser(util.UUIDToString(id), email.String)
}

func (s *AdminCollaborationService) read(ctx context.Context, organization pgtype.UUID, read func(*db.Queries, PlatformAdminIdentity) error) error {
	if _, err := s.authorize(ctx, s.Queries, organization); err != nil {
		return err
	}
	if s.TxStarter == nil {
		return platformError(503, "admin_unavailable", "Collaboration administration is unavailable")
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollbackCollaborationQueue(ctx, tx)
	if _, err = tx.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY"); err != nil {
		return err
	}
	q := s.Queries.WithTx(tx)
	actor, err := s.authorize(ctx, q, organization)
	if err != nil {
		return err
	}
	if err = read(q, actor); err != nil {
		return adminCollaborationServiceError(err)
	}
	return tx.Commit(ctx)
}

func validateAdminCollaborationPage(p AdminCollaborationPageQuery) error {
	if p.Limit < 1 || p.Limit > 100 {
		return platformError(400, "invalid_page_limit", "The page limit must be between 1 and 100")
	}
	return nil
}

func (s *AdminCollaborationService) ListIssues(ctx context.Context, organization pgtype.UUID, p AdminCollaborationPageQuery) (AdminCollaborationPage, error) {
	out := AdminCollaborationPage{Items: make([]AdminCollaborationIssue, 0)}
	err := s.read(ctx, organization, func(q *db.Queries, actor PlatformAdminIdentity) error {
		out.Scope = actor.OrganizationID
		if err := validateAdminCollaborationPage(p); err != nil {
			return err
		}
		disabledIDs, disabledEmails := auth.TemporarilyDisabledUsers()
		rows, err := q.ListAdminCollaborationIssues(ctx, db.ListAdminCollaborationIssuesParams{
			OrganizationID: organization, AfterID: p.Cursor, PageLimit: int32(p.Limit + 1),
			TemporarilyDisabledUserIds: disabledIDs, TemporarilyDisabledUserEmails: disabledEmails,
		})
		if err != nil {
			return err
		}
		if len(rows) > p.Limit {
			rows, out.NextCursor = rows[:p.Limit], rows[p.Limit-1].IssueID
		}
		for _, row := range rows {
			out.Items = append(out.Items, adminCollaborationIssueView(db.GetAdminCollaborationIssueRow(row)))
		}
		return nil
	})
	if err != nil {
		return AdminCollaborationPage{}, err
	}
	return out, nil
}

func (s *AdminCollaborationService) GetIssue(ctx context.Context, organization, issueID pgtype.UUID) (AdminCollaborationDetail, error) {
	var out AdminCollaborationDetail
	err := s.read(ctx, organization, func(q *db.Queries, actor PlatformAdminIdentity) error {
		out.Scope = actor.OrganizationID
		if !issueID.Valid {
			return platformError(400, "invalid_issue_id", "A collaboration issue ID is required")
		}
		row, err := q.GetAdminCollaborationIssue(ctx, db.GetAdminCollaborationIssueParams{OrganizationID: organization, IssueID: issueID})
		if err != nil {
			return err
		}
		out.AdminCollaborationIssue = adminCollaborationIssueView(row)
		if !out.Creator.Disabled {
			return platformError(409, "creator_not_disabled", "Only a disabled creator permits lifecycle administration")
		}
		out.Actions = make([]CollaborationStatusOptions, 0, 2)
		var actions []string
		if row.QueueState == "closed" && collaboration.TerminalCategory(row.StatusCategory) {
			actions = []string{"reopen"}
			out.SuccessorRequired = out.Handler.Disabled
		} else if (row.QueueState == "active" || row.QueueState == "paused") && collaboration.NonterminalCategory(row.StatusCategory) {
			actions = []string{"complete", "cancel"}
		}
		catalog, err := q.ListIssueStatusEntries(ctx, db.ListIssueStatusEntriesParams{WorkspaceID: row.SourceWorkspaceID, IncludeArchived: true})
		if err != nil {
			return err
		}
		// Status projection needs only the existing source key, not the private
		// task content or an invented ordinary collaboration capability.
		issue := db.Issue{ID: row.IssueID, WorkspaceID: row.SourceWorkspaceID, Status: row.Status}
		for _, action := range actions {
			options, err := collaborationStatusOptions(issue, action, catalog, db.IssueHandoff{})
			var statusErr *CollaborationError
			if errors.As(err, &statusErr) && statusErr.Code == "status_unavailable" {
				continue
			}
			if err != nil {
				return err
			}
			out.Actions = append(out.Actions, options)
		}
		return nil
	})
	if err != nil {
		return AdminCollaborationDetail{}, err
	}
	return out, nil
}

func (s *AdminCollaborationService) ReopenCandidates(ctx context.Context, organization, issueID pgtype.UUID, p AdminCollaborationPageQuery) (AdminCollaborationCandidatePage, error) {
	out := AdminCollaborationCandidatePage{Items: make([]CollaborationContentAccount, 0)}
	err := s.read(ctx, organization, func(q *db.Queries, actor PlatformAdminIdentity) error {
		out.Scope = actor.OrganizationID
		if !issueID.Valid {
			return platformError(400, "invalid_issue_id", "A collaboration issue ID is required")
		}
		if err := validateAdminCollaborationPage(p); err != nil {
			return err
		}
		row, err := q.GetAdminCollaborationIssue(ctx, db.GetAdminCollaborationIssueParams{OrganizationID: organization, IssueID: issueID})
		if err != nil {
			return err
		}
		issue := adminCollaborationIssueView(row)
		if !issue.Creator.Disabled {
			return platformError(409, "creator_not_disabled", "Only a disabled creator permits lifecycle administration")
		}
		if row.QueueState != "closed" || !collaboration.TerminalCategory(row.StatusCategory) || !issue.Handler.Disabled {
			return platformError(409, "successor_not_required", "Candidates are available only for a closed collaboration with both accounts disabled")
		}
		out.IssueID, out.CollaborationRevision = issueID, row.CollaborationRevision
		query := db.ListAdminCollaborationReopenCandidatesParams{
			IssueID: issueID, SourceWorkspaceID: row.SourceWorkspaceID, AfterID: p.Cursor, PageLimit: int32(p.Limit + 1),
		}
		// The emergency denylist is enforced outside SQL. Fill the visible page
		// in this same authorized snapshot before choosing its continuation.
		for {
			rows, err := q.ListAdminCollaborationReopenCandidates(ctx, query)
			if err != nil {
				return err
			}
			for _, candidate := range rows {
				if auth.IsTemporarilyDisabledUser(util.UUIDToString(candidate.UserID), candidate.Email.String) {
					continue
				}
				if len(out.Items) == p.Limit {
					out.NextCursor = out.Items[len(out.Items)-1].UserID
					return nil
				}
				out.Items = append(out.Items, CollaborationContentAccount{UserID: candidate.UserID, Name: candidate.Name, AvatarURL: candidate.AvatarUrl, External: candidate.External})
			}
			if len(rows) < int(query.PageLimit) {
				return nil
			}
			query.AfterID = rows[len(rows)-1].UserID
		}
	})
	if err != nil {
		return AdminCollaborationCandidatePage{}, err
	}
	return out, nil
}

type adminCollaborationStoredResult struct {
	OperationID pgtype.UUID               `json:"operation_id"`
	Result      AdminCollaborationReceipt `json:"result"`
}

func loadAdminCollaborationReceipt(ctx context.Context, q *db.Queries, op db.AdminOperation) (AdminCollaborationReceipt, error) {
	var out AdminCollaborationReceipt
	action := strings.TrimPrefix(op.Kind, "collaboration.")
	if op.TargetKind != "collaboration_issue" || !strings.HasPrefix(op.Kind, "collaboration.") || adminCollaborationCode(action) == "" {
		return out, pgx.ErrNoRows
	}
	record, err := q.GetCollaborationOperation(ctx, db.GetCollaborationOperationParams{ScopeKind: "issue", ScopeID: op.TargetID,
		ActorUserID: op.ActorID, RequestID: util.UUIDToString(op.IdempotencyKey)})
	if err != nil {
		return out, err
	}
	var stored adminCollaborationStoredResult
	if !record.CompletedAt.Valid || record.ResolvedIssueID != op.TargetID || record.Kind != "admin_"+action || record.PayloadHash != op.PayloadHash ||
		record.ResultCode.String != op.ResultCode || op.ResultCode != adminCollaborationCode(action) || op.State != "applied" ||
		json.Unmarshal(record.Result, &stored) != nil || stored.OperationID != op.ID {
		return out, platformError(503, "admin_operation_unavailable", "The original administration result is unavailable")
	}
	out = stored.Result
	expectedQueue := "closed"
	if action == "reopen" {
		expectedQueue = "paused"
	}
	if out.IssueID != op.TargetID || out.RequestID != util.UUIDToString(op.IdempotencyKey) || out.Action != action || out.Code != op.ResultCode ||
		out.CollaborationRevision < 1 || out.ControlVersion < 1 || out.QueueState != expectedQueue || out.UnsettledRuns < 0 ||
		out.PreviousStatus == "" || out.Status == "" || !out.CreatorUserID.Valid || !out.PreviousHandlerUserID.Valid || !out.HandlerUserID.Valid ||
		(action != "reopen" && out.HandlerUserID != out.PreviousHandlerUserID) {
		return AdminCollaborationReceipt{}, platformError(503, "admin_operation_unavailable", "The original administration result is unavailable")
	}
	return out, nil
}

func (s *AdminCollaborationService) GetOperation(ctx context.Context, organization, issueID, key pgtype.UUID) (AdminCollaborationResult, error) {
	var out AdminCollaborationResult
	err := s.read(ctx, organization, func(q *db.Queries, actor PlatformAdminIdentity) error {
		if !issueID.Valid || !key.Valid {
			return platformError(400, "invalid_operation", "An issue and original request key are required")
		}
		// A restored creator does not erase the original result. Only current
		// management authority/source scope and the caller's own key permit it.
		row, err := q.GetAdminCollaborationIssue(ctx, db.GetAdminCollaborationIssueParams{OrganizationID: organization, IssueID: issueID})
		if err != nil {
			return err
		}
		op, err := q.GetAdminOperationByKey(ctx, db.GetAdminOperationByKeyParams{OrganizationID: organization, ActorID: actor.UserID, IdempotencyKey: key})
		if err != nil {
			return err
		}
		if op.TargetID != issueID {
			return pgx.ErrNoRows
		}
		if op.Kind == "collaboration."+adminCollaborationMemoryPurge {
			memory, err := loadAdminCollaborationMemoryReceipt(ctx, q, op)
			if err != nil {
				return err
			}
			if memory.CreatorUserID != row.CreatorUserID {
				return platformError(503, "admin_operation_unavailable", "The original administration identity is unavailable")
			}
			out = AdminCollaborationResult{Operation: op, MemoryResult: &memory, Replayed: true}
			return nil
		}
		receipt, err := loadAdminCollaborationReceipt(ctx, q, op)
		if err != nil {
			return err
		}
		if receipt.CreatorUserID != row.CreatorUserID {
			return platformError(503, "admin_operation_unavailable", "The original administration identity is unavailable")
		}
		out = AdminCollaborationResult{Operation: op, Result: receipt, Replayed: true}
		return nil
	})
	if err != nil {
		return AdminCollaborationResult{}, err
	}
	return out, nil
}
