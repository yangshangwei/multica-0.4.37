package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/iteration"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type CollaborationLifecycleParams struct {
	CollaborationWorkflowRequest
	Action               string
	TargetStatus         string
	StatusOptionsVersion string
}

type CollaborationStatusParams struct {
	CollaborationWorkflowRequest
	TargetStatus         string
	StatusOptionsVersion string
}

type CollaborationStatusOption struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

type CollaborationStatusOptions struct {
	Action               string                      `json:"action"`
	StatusOptionsVersion string                      `json:"status_options_version"`
	DefaultStatus        string                      `json:"default_status"`
	Options              []CollaborationStatusOption `json:"options"`
}

func collaborationStatusAction(action string) bool {
	return action == "complete" || action == "cancel" || action == "reopen" || action == "change_status" || collaborationDirectedStatusAction(action)
}

func collaborationDirectedStatusAction(action string) bool {
	return action == "request_information" || action == "return_information" || action == "submit_acceptance"
}

func collaborationStatusCategoryAllowed(action, category string) bool {
	switch action {
	case "complete":
		return category == "done"
	case "cancel":
		return category == "cancelled"
	case "reopen", "change_status", "return_information":
		return collaboration.NonterminalCategory(category)
	case "request_information":
		return category == "blocked"
	case "submit_acceptance":
		return category == "in_review"
	default:
		return false
	}
}

func collaborationStatusActionEligible(c collaboration.Capabilities, action string) bool {
	switch action {
	case "complete":
		return c.Complete
	case "cancel":
		return c.Cancel
	case "reopen":
		return c.Reopen
	case "change_status":
		return c.ChangeStatus
	case "request_information":
		return c.RequestInformation
	case "return_information":
		return c.ReturnInformation
	case "submit_acceptance":
		return c.SubmitAcceptance
	default:
		return false
	}
}

// Options are an action projection, not permission to browse a source catalog.
// A read-only repeatable snapshot keeps authority, status and catalog coherent;
// the mutation rechecks all of them under the catalog/source locks.
func (s *CollaborationService) StatusOptions(ctx context.Context, issueID pgtype.UUID, action string) (CollaborationStatusOptions, error) {
	principal, ok := collaboration.PrincipalFromContext(ctx)
	if !ok || !principal.Human() {
		return CollaborationStatusOptions{}, collaborationError(http.StatusNotFound, "not_found")
	}
	if !issueID.Valid || !collaborationStatusAction(action) {
		return CollaborationStatusOptions{}, collaborationError(http.StatusBadRequest, "invalid_status_request")
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return CollaborationStatusOptions{}, err
	}
	defer rollbackCollaborationQueue(ctx, tx)
	if _, err = tx.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY"); err != nil {
		return CollaborationStatusOptions{}, err
	}
	q := s.Queries.WithTx(tx)
	access, err := s.ResolveIssueAccess(ctx, q, issueID)
	if err != nil {
		return CollaborationStatusOptions{}, err
	}
	if !collaborationStatusActionEligible(access.Capabilities, action) {
		return CollaborationStatusOptions{}, collaborationError(http.StatusForbidden, "forbidden")
	}
	catalog, err := q.ListIssueStatusEntries(ctx, db.ListIssueStatusEntriesParams{WorkspaceID: access.Issue.WorkspaceID, IncludeArchived: true})
	if err != nil {
		return CollaborationStatusOptions{}, err
	}
	options, err := collaborationActionStatusOptions(ctx, q, access, action, catalog)
	if err != nil {
		return CollaborationStatusOptions{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return CollaborationStatusOptions{}, err
	}
	return options, nil
}

func collaborationActionStatusOptions(ctx context.Context, q *db.Queries, access CollaborationIssueAccess, action string, catalog []db.IssueStatus) (CollaborationStatusOptions, error) {
	var request db.IssueHandoff
	if action == "return_information" {
		var open bool
		var err error
		request, open, err = collaborationInformationRequest(ctx, q, access)
		if err != nil {
			return CollaborationStatusOptions{}, err
		}
		if !open {
			return CollaborationStatusOptions{}, collaborationError(http.StatusConflict, "information_request_not_active")
		}
	}
	return collaborationStatusOptions(access.Issue, action, catalog, request)
}

func collaborationStatusOptions(issue db.Issue, action string, catalog []db.IssueStatus, request db.IssueHandoff) (CollaborationStatusOptions, error) {
	options := CollaborationStatusOptions{Action: action, Options: make([]CollaborationStatusOption, 0)}
	canonical, fallback, available := "", false, true
	switch action {
	case "complete":
		options.DefaultStatus = "done"
	case "cancel":
		options.DefaultStatus = "cancelled"
	case "request_information":
		canonical = "blocked"
		options.DefaultStatus, available = collaborationHandoffStatus(catalog, issue.Status, canonical)
	case "submit_acceptance":
		canonical = "in_review"
		options.DefaultStatus, available = collaborationHandoffStatus(catalog, issue.Status, canonical)
	case "return_information":
		canonical = "in_progress"
		options.DefaultStatus, fallback, available = collaborationReturnStatus(catalog, request.PreviousStatus)
	default:
		options.DefaultStatus = "todo"
	}
	if !available {
		return CollaborationStatusOptions{}, collaborationError(http.StatusConflict, "status_unavailable")
	}
	// Hash immutable identities and mutable display/freshness facts, including
	// archived choices. Sorting by key is independent of query display order.
	type catalogFact struct {
		Key, Name, Category   string
		ArchivedAt, UpdatedAt pgtype.Timestamptz
	}
	facts := make([]catalogFact, 0, len(catalog))
	currentCategory := ""
	for _, entry := range catalog {
		if entry.Key == issue.Status {
			currentCategory = entry.Category
		}
		relevant := collaborationStatusCategoryAllowed(action, entry.Category)
		if collaborationDirectedStatusAction(action) {
			// Only these entries decide the prescribed target. Include archived
			// and recategorized saved keys even when they are no longer eligible.
			relevant = entry.Key == issue.Status || entry.Key == canonical || (action == "return_information" && entry.Key == request.PreviousStatus)
		}
		if !relevant {
			continue
		}
		facts = append(facts, catalogFact{entry.Key, entry.Name, entry.Category, entry.ArchivedAt, entry.UpdatedAt})
		if !entry.ArchivedAt.Valid && collaborationStatusCategoryAllowed(action, entry.Category) && (!collaborationDirectedStatusAction(action) || entry.Key == options.DefaultStatus) {
			options.Options = append(options.Options, CollaborationStatusOption{Key: entry.Key, Name: entry.Name, Category: entry.Category})
			if action == "change_status" && entry.Key == issue.Status {
				options.DefaultStatus = issue.Status
			}
		}
	}
	if !collaboration.NonterminalCategory(currentCategory) && !collaboration.TerminalCategory(currentCategory) {
		return CollaborationStatusOptions{}, collaborationError(http.StatusConflict, "status_unavailable")
	}
	sort.Slice(facts, func(i, j int) bool { return facts[i].Key < facts[j].Key })
	sort.Slice(options.Options, func(i, j int) bool { return options.Options[i].Key < options.Options[j].Key })
	defaultFound := false
	for _, option := range options.Options {
		defaultFound = defaultFound || option.Key == options.DefaultStatus
	}
	if !defaultFound {
		return CollaborationStatusOptions{}, collaborationError(http.StatusConflict, "status_unavailable")
	}
	// Returning the same status through another layer is still a different
	// confirmation: bind the digest to the saved request, not just its key.
	type requestFact struct {
		ID, Requester, Recipient, Parent pgtype.UUID
		PreviousStatus                   string
	}
	var savedRequest *requestFact
	if action == "return_information" {
		savedRequest = &requestFact{request.ID, request.RequesterUserID, request.ToUserID, request.ParentRequestID, request.PreviousStatus}
	}
	encoded, err := json.Marshal(struct {
		SourceWorkspaceID                                     pgtype.UUID
		Action, CurrentStatus, CurrentCategory, DefaultStatus string
		Catalog                                               []catalogFact
		SavedRequest                                          *requestFact
		Fallback                                              bool
	}{issue.WorkspaceID, action, issue.Status, currentCategory, options.DefaultStatus, facts, savedRequest, fallback})
	if err != nil {
		return CollaborationStatusOptions{}, err
	}
	digest := sha256.Sum256(encoded)
	options.StatusOptionsVersion = hex.EncodeToString(digest[:])
	return options, nil
}

func validateCollaborationStatusInput(target, version string) bool {
	digest, err := hex.DecodeString(version)
	return strings.TrimSpace(target) != "" && len(target) <= 128 && err == nil && len(digest) == sha256.Size && version == strings.ToLower(version)
}

// Supplemental choices describe current authority, not a historical receipt.
// Replaying a refusal retains its original result and never reexecutes the
// mutation, even if the original target has become valid in the meantime.
func (t *collaborationWorkflowTx) projectStatusOptions(ctx context.Context, action string, receipt CollaborationWorkflowReceipt) (CollaborationWorkflowReceipt, error) {
	receipt.StatusOptions = nil
	if !collaborationStatusActionEligible(t.access.Capabilities, action) {
		return receipt, nil
	}
	options, err := collaborationActionStatusOptions(ctx, t.q, t.access, action, t.catalog)
	if err != nil {
		if e, ok := err.(*CollaborationError); ok && e.Code == "status_unavailable" {
			return receipt, nil
		}
		return CollaborationWorkflowReceipt{}, err
	}
	receipt.StatusOptions = &options
	return receipt, nil
}

func (t *collaborationWorkflowTx) validateStatus(ctx context.Context, action, target, version string) (*CollaborationWorkflowReceipt, error) {
	if !validateCollaborationStatusInput(target, version) {
		receipt := t.result("invalid_status_request", http.StatusBadRequest)
		return &receipt, nil
	}
	options, err := collaborationActionStatusOptions(ctx, t.q, t.access, action, t.catalog)
	if err != nil {
		if e, ok := err.(*CollaborationError); ok {
			receipt := t.result(e.Code, e.Status)
			return &receipt, nil
		}
		return nil, err
	}
	if options.StatusOptionsVersion != version || (collaborationDirectedStatusAction(action) && options.DefaultStatus != target) {
		receipt := t.result("status_options_changed", http.StatusConflict)
		receipt.StatusOptions = &options
		return &receipt, nil
	}
	for _, option := range options.Options {
		if option.Key == target {
			return nil, nil
		}
	}
	receipt := t.result("status_unavailable", http.StatusConflict)
	return &receipt, nil
}

func (s *CollaborationService) ChangeLifecycle(ctx context.Context, p CollaborationLifecycleParams) (CollaborationWorkflowReceipt, error) {
	// Let the common owner reject machine principals before dependencies, even
	// when their request body is malformed.
	return s.mutateCollaborationWorkflow(ctx, p.CollaborationWorkflowRequest, pgtype.UUID{}, p.Action, p, func(t *collaborationWorkflowTx) (CollaborationWorkflowReceipt, error) {
		if p.Action != "complete" && p.Action != "cancel" && p.Action != "reopen" || !validateCollaborationStatusInput(p.TargetStatus, p.StatusOptionsVersion) {
			return t.result("invalid_status_request", http.StatusBadRequest), nil
		}
		if p.Action == "reopen" {
			if !collaboration.TerminalCategory(t.access.StatusCategory) || t.session.QueueState != "closed" {
				return t.result("issue_not_closed", http.StatusConflict), nil
			}
		} else if collaboration.TerminalCategory(t.access.StatusCategory) || t.session.QueueState == "closed" {
			return t.result("issue_closed", http.StatusConflict), nil
		} else if !collaboration.NonterminalCategory(t.access.StatusCategory) {
			return t.result("status_unavailable", http.StatusConflict), nil
		}
		if refusal, err := t.validateStatus(ctx, p.Action, p.TargetStatus, p.StatusOptionsVersion); refusal != nil || err != nil {
			if refusal != nil {
				return *refusal, nil
			}
			return CollaborationWorkflowReceipt{}, err
		}
		return t.changeLifecycle(ctx, p.Action, p.TargetStatus)
	})
}

func (s *CollaborationService) ChangeStatus(ctx context.Context, p CollaborationStatusParams) (CollaborationWorkflowReceipt, error) {
	return s.mutateCollaborationWorkflow(ctx, p.CollaborationWorkflowRequest, pgtype.UUID{}, "change_status", p, func(t *collaborationWorkflowTx) (CollaborationWorkflowReceipt, error) {
		if !validateCollaborationStatusInput(p.TargetStatus, p.StatusOptionsVersion) {
			return t.result("invalid_status_request", http.StatusBadRequest), nil
		}
		if collaboration.TerminalCategory(t.access.StatusCategory) || t.session.QueueState == "closed" {
			return t.result("issue_closed", http.StatusConflict), nil
		}
		if !collaboration.NonterminalCategory(t.access.StatusCategory) {
			return t.result("status_unavailable", http.StatusConflict), nil
		}
		if refusal, err := t.validateStatus(ctx, "change_status", p.TargetStatus, p.StatusOptionsVersion); refusal != nil || err != nil {
			if refusal != nil {
				return *refusal, nil
			}
			return CollaborationWorkflowReceipt{}, err
		}
		if t.access.Issue.Status == p.TargetStatus {
			return t.result("status_unchanged", http.StatusOK), nil
		}
		previous := t.access.Issue.Status
		if err := t.recordStatusChange(ctx, p.TargetStatus); err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		var err error
		t.access.Revision, err = t.q.AdvanceCollaborationContentRevision(ctx, t.access.Issue.ID)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		return t.finishStatusChange(ctx, "status_changed", previous, nil)
	})
}

func (t *collaborationWorkflowTx) changeLifecycle(ctx context.Context, action, target string) (CollaborationWorkflowReceipt, error) {
	previous := t.access.Issue.Status
	code := "reopened"
	var stopped *CollaborationQueueReceipt
	if action == "reopen" {
		session, err := t.q.ReopenCollaborationSession(ctx, t.session.ID)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		t.session = session
		for _, item := range t.targets {
			if _, err = t.pauseTarget(ctx, item, "issue_reopened"); err != nil {
				return CollaborationWorkflowReceipt{}, err
			}
		}
	} else {
		code = "completed"
		if action == "cancel" {
			code = "cancelled"
		}
		reason := "issue_" + code
		var err error
		t.session, err = t.q.CloseCollaborationSession(ctx, db.CloseCollaborationSessionParams{SessionID: t.session.ID, Reason: reason})
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if err = t.q.CancelCollaborationWaitingCommands(ctx, db.CancelCollaborationWaitingCommandsParams{SessionID: t.session.ID, Reason: reason}); err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		selected := make(map[pgtype.UUID]string, len(t.targets))
		for _, item := range t.targets {
			selected[item.ID] = "cancelled"
		}
		receipt, err := t.stopTargets(ctx, selected, true, reason, true)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		stopped = &receipt
	}
	if err := t.recordStatusChange(ctx, target); err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	if err := t.q.SupersedeCollaborationInformationRequests(ctx, t.access.Issue.ID); err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	if err := t.q.ClearCollaborationInformationRequest(ctx, t.access.Issue.ID); err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	state, err := t.q.AdvanceCollaborationLifecycle(ctx, t.access.Issue.ID)
	if err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	t.access.Revision, t.access.LifecycleEpoch = state.Revision, state.LifecycleEpoch
	return t.finishStatusChange(ctx, code, previous, stopped)
}

func (t *collaborationWorkflowTx) recordStatusChange(ctx context.Context, target string) error {
	record, err := iteration.PrepareIssueRecord(ctx, t.tx, t.access.Issue, t.iteration)
	if err != nil {
		return err
	}
	after, err := t.q.UpdateIssueStatus(ctx, db.UpdateIssueStatusParams{ID: t.access.Issue.ID, WorkspaceID: t.access.Issue.WorkspaceID, Status: target})
	if err != nil {
		return err
	}
	actor, err := json.Marshal(map[string]any{"type": "member", "id": t.actor, "user_id": t.actor})
	if err != nil {
		return err
	}
	if err = iteration.RecordIssueChange(ctx, t.tx, record, after, actor, t.operation.ID); err != nil {
		return err
	}
	t.access.Issue = after
	return nil
}

func (t *collaborationWorkflowTx) finishStatusChange(ctx context.Context, code, previous string, stopped *CollaborationQueueReceipt) (CollaborationWorkflowReceipt, error) {
	receipt := t.result(code, http.StatusOK)
	if stopped != nil {
		receipt.RunID, receipt.RunSettled, receipt.Execution = stopped.RunID, stopped.RunSettled, stopped
	}
	seq, err := t.q.AdvanceCollaborationEventSequence(ctx, t.access.Issue.ID)
	if err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	payload, err := json.Marshal(map[string]any{"previous_status": previous, "status": t.access.Issue.Status, "receipt": receipt, "request_id": t.operation.RequestID})
	if err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	// R52 does not subscribe users to status or lifecycle edits. The durable
	// event remains the history/outbox fact, without creating inbox work.
	_, err = t.q.CreateCollaborationEvent(ctx, db.CreateCollaborationEventParams{IssueID: t.access.Issue.ID, EventSeq: seq, ActorUserID: t.actor, Kind: code, Payload: payload})
	return receipt, err
}
