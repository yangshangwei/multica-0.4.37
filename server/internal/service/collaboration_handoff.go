package service

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Directed handoffs (D04): request information, return it to the account that
// asked, and submit the stage result to the creator for acceptance. None of
// them completes the issue, changes queue/run state or dispatches AI work.

type CollaborationInformationRequestParams struct {
	CollaborationTransferParams
	ExpectedStatus       string
	StatusOptionsVersion string
}

type CollaborationInformationReturnParams struct {
	CollaborationWorkflowRequest
	InformationRequestID pgtype.UUID
	// ExpectedStatus is the restore target the client displayed. A different
	// server result is refused instead of silently applied.
	ExpectedStatus       string
	StatusOptionsVersion string
	Note                 string
}

type CollaborationAcceptanceParams struct {
	CollaborationWorkflowRequest
	ExpectedStatus       string
	StatusOptionsVersion string
	Note                 string
}

// CollaborationInformationRequestView is the detail projection of the open top
// of the request chain. ReturnStatus is empty when the source has no valid
// restore target; ReturnStatusFallback marks a replaced saved status.
type CollaborationInformationRequestView struct {
	ID                   pgtype.UUID
	RequesterID          pgtype.UUID
	RecipientID          pgtype.UUID
	ParentRequestID      pgtype.UUID
	Note                 string
	PreviousStatus       string
	ReturnStatus         string
	ReturnStatusFallback bool
	CreatedAt            pgtype.Timestamptz
}

func collaborationCatalogEntry(catalog []db.IssueStatus, key string) (db.IssueStatus, bool) {
	for _, entry := range catalog {
		if entry.Key == key {
			return entry, true
		}
	}
	return db.IssueStatus{}, false
}

// collaborationHandoffStatus keeps a current key that already has the wanted
// category and otherwise selects that category's canonical source key. It never
// borrows a same-named key from another workspace or invents a default.
func collaborationHandoffStatus(catalog []db.IssueStatus, current, category string) (string, bool) {
	if entry, ok := collaborationCatalogEntry(catalog, current); ok && entry.Category == category && !entry.ArchivedAt.Valid {
		return current, true
	}
	if entry, ok := collaborationCatalogEntry(catalog, category); ok && entry.Category == category && !entry.ArchivedAt.Valid {
		return category, true
	}
	return "", false
}

// collaborationReturnStatus restores the request's saved key while it remains a
// selectable nonterminal source status, else the canonical in_progress key.
func collaborationReturnStatus(catalog []db.IssueStatus, saved string) (string, bool, bool) {
	if entry, ok := collaborationCatalogEntry(catalog, saved); ok && !entry.ArchivedAt.Valid && collaboration.NonterminalCategory(entry.Category) {
		return saved, false, true
	}
	if entry, ok := collaborationCatalogEntry(catalog, "in_progress"); ok && entry.Category == "in_progress" && !entry.ArchivedAt.Valid {
		return "in_progress", true, true
	}
	return "", false, false
}

func (t *collaborationWorkflowTx) handler() pgtype.UUID {
	if !t.access.Issue.AssigneeType.Valid || t.access.Issue.AssigneeType.String != "member" {
		return pgtype.UUID{}
	}
	return t.access.Issue.AssigneeID
}

func (t *collaborationWorkflowTx) openForHandoff() (*CollaborationWorkflowReceipt, bool) {
	if collaboration.TerminalCategory(t.access.StatusCategory) || t.session.QueueState == "closed" {
		r := t.result("issue_closed", http.StatusConflict)
		return &r, false
	}
	if !collaboration.NonterminalCategory(t.access.StatusCategory) {
		r := t.result("status_unavailable", http.StatusConflict)
		return &r, false
	}
	return nil, true
}

func (t *collaborationWorkflowTx) activeParticipant(ctx context.Context, id pgtype.UUID) (bool, error) {
	participant, err := t.q.GetCollaborationParticipant(ctx, db.GetCollaborationParticipantParams{IssueID: t.access.Issue.ID, UserID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil && !participant.RevokedAt.Valid, err
}

// activeInformationRequest loads the open top of the chain under the
// issue_collaboration lock. A dangling pointer is treated as no request.
func (t *collaborationWorkflowTx) activeInformationRequest(ctx context.Context) (db.IssueHandoff, bool, error) {
	return collaborationInformationRequest(ctx, t.q, t.access)
}

func collaborationInformationRequest(ctx context.Context, q *db.Queries, access CollaborationIssueAccess) (db.IssueHandoff, bool, error) {
	if !access.InformationRequestID.Valid {
		return db.IssueHandoff{}, false, nil
	}
	request, err := q.GetCollaborationInformationRequest(ctx, db.GetCollaborationInformationRequestParams{IssueID: access.Issue.ID, RequestID: access.InformationRequestID})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.IssueHandoff{}, false, nil
	}
	if err != nil {
		return db.IssueHandoff{}, false, err
	}
	return request, request.Resolution == "open", nil
}

func (s *CollaborationService) RequestInformation(ctx context.Context, p CollaborationInformationRequestParams) (CollaborationWorkflowReceipt, error) {
	return s.mutateCollaborationWorkflow(ctx, p.CollaborationWorkflowRequest, p.TargetUserID, "request_information", p, func(t *collaborationWorkflowTx) (CollaborationWorkflowReceipt, error) {
		if !t.access.Capabilities.Transfer {
			return t.result("forbidden", http.StatusForbidden), nil
		}
		if refusal, ok := t.openForHandoff(); !ok {
			return *refusal, nil
		}
		if p.TargetUserID == t.actor || p.TargetUserID == t.handler() {
			return t.result("invalid_information_target", http.StatusConflict), nil
		}
		if !t.validTarget(p.TargetUserID) {
			return t.result("target_unavailable", http.StatusConflict), nil
		}
		if refusal, err := t.validateStatus(ctx, "request_information", p.ExpectedStatus, p.StatusOptionsVersion); refusal != nil || err != nil {
			if refusal != nil {
				return *refusal, nil
			}
			return CollaborationWorkflowReceipt{}, err
		}
		active, open, err := t.activeInformationRequest(ctx)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		// The current recipient may push a nested request; the chain then
		// returns layer by layer. Any other requester starts a new chain.
		var parent pgtype.UUID
		if open && active.ToUserID == t.actor && t.handler() == t.actor {
			parent = active.ID
		} else if err = t.supersedeInformation(ctx); err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if _, err = t.enroll(ctx, p.TargetUserID); err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		request, err := t.handoff(ctx, collaborationHandoff{target: p.TargetUserID, purpose: "information", note: p.Note,
			status: p.ExpectedStatus, requester: t.actor, parent: parent})
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if err = t.q.SetCollaborationInformationRequest(ctx, db.SetCollaborationInformationRequestParams{IssueID: t.access.Issue.ID, RequestID: request.ID}); err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		t.access.InformationRequestID = request.ID
		return t.finishWith(ctx, "information_requested", p.TargetUserID, nil, map[string]any{
			"information_request_id": request.ID, "parent_request_id": parent, "status": t.access.Issue.Status})
	})
}

func (s *CollaborationService) ReturnInformation(ctx context.Context, p CollaborationInformationReturnParams) (CollaborationWorkflowReceipt, error) {
	return s.mutateCollaborationWorkflow(ctx, p.CollaborationWorkflowRequest, pgtype.UUID{}, "return_information", p, func(t *collaborationWorkflowTx) (CollaborationWorkflowReceipt, error) {
		if !p.InformationRequestID.Valid {
			return t.result("invalid_information_return", http.StatusBadRequest), nil
		}
		if refusal, ok := t.openForHandoff(); !ok {
			return *refusal, nil
		}
		active, open, err := t.activeInformationRequest(ctx)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		// Only the current recipient may answer the open top of the chain. An
		// older, consumed or superseded request can never move responsibility.
		if !open || active.ID != p.InformationRequestID || active.ToUserID != t.actor || t.handler() != t.actor {
			return t.result("information_request_not_active", http.StatusConflict), nil
		}
		requester := active.RequesterUserID
		participant, err := t.activeParticipant(ctx, requester)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		// A departed or disabled requester is not re-invited; someone with
		// transfer authority chooses an ordinary transfer instead.
		if !requester.Valid || !participant || !t.validTarget(requester) {
			return t.result("return_target_unavailable", http.StatusConflict), nil
		}
		if refusal, err := t.validateStatus(ctx, "return_information", p.ExpectedStatus, p.StatusOptionsVersion); refusal != nil || err != nil {
			if refusal != nil {
				return *refusal, nil
			}
			return CollaborationWorkflowReceipt{}, err
		}
		returned, err := t.q.ReturnCollaborationInformationRequest(ctx, db.ReturnCollaborationInformationRequestParams{IssueID: t.access.Issue.ID, RequestID: active.ID})
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if _, err = t.handoff(ctx, collaborationHandoff{target: requester, purpose: "information_return", note: p.Note,
			status: p.ExpectedStatus, parent: returned.ID}); err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		// Restore the parent request when this was a nested layer. The chain
		// invariant keeps every open ancestor addressed to the next requester.
		next := pgtype.UUID{}
		if returned.ParentRequestID.Valid {
			parent, err := t.q.GetCollaborationInformationRequest(ctx, db.GetCollaborationInformationRequestParams{IssueID: t.access.Issue.ID, RequestID: returned.ParentRequestID})
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return CollaborationWorkflowReceipt{}, err
			}
			if err == nil && parent.Resolution == "open" && parent.ToUserID == requester {
				next = parent.ID
			}
		}
		if err = t.q.SetCollaborationInformationRequest(ctx, db.SetCollaborationInformationRequestParams{IssueID: t.access.Issue.ID, RequestID: next}); err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		t.access.InformationRequestID = next
		return t.finishWith(ctx, "information_returned", requester, nil, map[string]any{
			"information_request_id": returned.ID, "active_information_request_id": next, "status": t.access.Issue.Status})
	})
}

func (s *CollaborationService) SubmitAcceptance(ctx context.Context, p CollaborationAcceptanceParams) (CollaborationWorkflowReceipt, error) {
	return s.mutateCollaborationWorkflow(ctx, p.CollaborationWorkflowRequest, pgtype.UUID{}, "submit_acceptance", p, func(t *collaborationWorkflowTx) (CollaborationWorkflowReceipt, error) {
		if t.handler() != t.actor {
			return t.result("forbidden", http.StatusForbidden), nil
		}
		creator := t.access.Issue.CreatorID
		if t.access.Issue.CreatorType != "member" || !creator.Valid {
			return t.result("target_unavailable", http.StatusConflict), nil
		}
		if creator == t.actor {
			return t.result("already_creator", http.StatusConflict), nil
		}
		if refusal, ok := t.openForHandoff(); !ok {
			return *refusal, nil
		}
		// A disabled creator cannot accept; R38 administration is separate and
		// never redirects acceptance to another account.
		participant, err := t.activeParticipant(ctx, creator)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if !participant || !t.validTarget(creator) {
			return t.result("target_unavailable", http.StatusConflict), nil
		}
		if refusal, err := t.validateStatus(ctx, "submit_acceptance", p.ExpectedStatus, p.StatusOptionsVersion); refusal != nil || err != nil {
			if refusal != nil {
				return *refusal, nil
			}
			return CollaborationWorkflowReceipt{}, err
		}
		if _, err := t.handoff(ctx, collaborationHandoff{target: creator, purpose: "acceptance", note: p.Note, status: p.ExpectedStatus}); err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if err := t.supersedeInformation(ctx); err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		return t.finishWith(ctx, "acceptance_submitted", creator, nil, map[string]any{"status": t.access.Issue.Status})
	})
}

// InformationRequestView projects the open top of the request chain for a
// reader of the issue. Writes recompute every fact under the workflow locks.
func (s *CollaborationService) InformationRequestView(ctx context.Context, access CollaborationIssueAccess) (*CollaborationInformationRequestView, error) {
	if !access.Capabilities.Read || !access.InformationRequestID.Valid {
		return nil, nil
	}
	request, err := s.Queries.GetCollaborationInformationRequest(ctx, db.GetCollaborationInformationRequestParams{IssueID: access.Issue.ID, RequestID: access.InformationRequestID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if request.Resolution != "open" || !request.RequesterUserID.Valid {
		return nil, nil
	}
	catalog, err := s.Queries.ListIssueStatusEntries(ctx, db.ListIssueStatusEntriesParams{WorkspaceID: access.Issue.WorkspaceID, IncludeArchived: true})
	if err != nil {
		return nil, err
	}
	status, fallback, _ := collaborationReturnStatus(catalog, request.PreviousStatus)
	return &CollaborationInformationRequestView{ID: request.ID, RequesterID: request.RequesterUserID, RecipientID: request.ToUserID,
		ParentRequestID: request.ParentRequestID, Note: request.Note, PreviousStatus: request.PreviousStatus,
		ReturnStatus: status, ReturnStatusFallback: fallback, CreatedAt: request.CreatedAt}, nil
}
