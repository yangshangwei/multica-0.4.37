package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/logger"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

// The human approval boundary for high-risk agent actions.
//
// What this is: an auditable record that a named person authorized one specific
// action before an agent took it, and a hard rule that only a person can create
// that record.
//
// What this is NOT: a sandbox. A task runs on a daemon host with that user's
// permissions, so an agent that ignores its policy can still run a command. The
// boundary this enforces is the one the server owns — no approval row, no
// authorization, and an execution recorded against an unapproved request is
// refused. Combined with the non-editable autonomy briefing the agent is given
// (service.AutonomyBriefing) and a runtime with least privilege, that is the
// enforceable part; the documentation says so rather than implying more.
//
// Everything here is deliberately small: a request, a human decision, and an
// execution record. Approvals are per-action, never standing grants.

const (
	maxApprovalSummaryLength = 500
	maxApprovalPlanLength    = 20000
	maxApprovalNoteLength    = 2000
	// Review queues are read by people and by an agent polling for its own answer.
	// Both want the recent ones; neither wants an unbounded scan.
	defaultApprovalPageSize = 50
	maxApprovalPageSize     = 200
)

// Audit actions written to activity_log alongside the approval row itself. The row
// carries the decision; these make it visible in the workspace's own timeline.
const (
	approvalActivityRequested = "agent_approval_requested"
	approvalActivityDecided   = "agent_approval_decided"
	approvalActivityExecuted  = "agent_approval_executed"
)

// AgentApprovalResponse is one approval request.
type AgentApprovalResponse struct {
	ID          string  `json:"id"`
	WorkspaceID string  `json:"workspace_id"`
	AgentID     string  `json:"agent_id"`
	TaskID      *string `json:"task_id"`
	IssueID     *string `json:"issue_id"`
	RiskClass   string  `json:"risk_class"`
	Summary     string  `json:"summary"`
	Plan        string  `json:"plan"`
	// Status is one of pending, approved, rejected, executed, cancelled. Clients
	// must treat an unrecognised value as "not approved".
	Status        string  `json:"status"`
	DecidedBy     *string `json:"decided_by"`
	DecidedAt     *string `json:"decided_at"`
	DecisionNote  string  `json:"decision_note"`
	ExecutedAt    *string `json:"executed_at"`
	ExecutionNote string  `json:"execution_note"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

func agentApprovalToResponse(a db.AgentApprovalRequest) AgentApprovalResponse {
	return AgentApprovalResponse{
		ID:            uuidToString(a.ID),
		WorkspaceID:   uuidToString(a.WorkspaceID),
		AgentID:       uuidToString(a.AgentID),
		TaskID:        uuidToPtr(a.TaskID),
		IssueID:       uuidToPtr(a.IssueID),
		RiskClass:     a.RiskClass,
		Summary:       a.Summary,
		Plan:          a.Plan,
		Status:        a.Status,
		DecidedBy:     uuidToPtr(a.DecidedBy),
		DecidedAt:     timestampToPtr(a.DecidedAt),
		DecisionNote:  a.DecisionNote,
		ExecutedAt:    timestampToPtr(a.ExecutedAt),
		ExecutionNote: a.ExecutionNote,
		CreatedAt:     timestampToString(a.CreatedAt),
		UpdatedAt:     timestampToString(a.UpdatedAt),
	}
}

// CreateAgentApprovalRequest is what an agent files before a high-risk action.
type CreateAgentApprovalRequestBody struct {
	// AgentID is required for a human filing on an agent's behalf and ignored for an
	// agent actor, whose identity comes from its task token rather than its request
	// body.
	AgentID   string `json:"agent_id"`
	IssueID   string `json:"issue_id"`
	RiskClass string `json:"risk_class"`
	Summary   string `json:"summary"`
	Plan      string `json:"plan"`
}

// CreateAgentApproval files one approval request for one action.
//
// Any actor may file one — asking permission is always allowed, and an agent below
// Operator that correctly recognises a production step needs somewhere to put it.
// What autonomy gates is EXECUTION (RecordAgentApprovalExecution), not the request.
func (h *Handler) CreateAgentApproval(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}

	var req CreateAgentApprovalRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.RiskClass = strings.TrimSpace(req.RiskClass)
	req.Summary = strings.TrimSpace(req.Summary)
	if !service.IsKnownApprovalRiskClass(req.RiskClass) {
		writeError(w, http.StatusBadRequest, "risk_class must be one of "+strings.Join(service.ApprovalRiskClasses, ", "))
		return
	}
	if req.Summary == "" {
		writeError(w, http.StatusBadRequest, "summary is required: say which single action needs approval")
		return
	}
	if utf8.RuneCountInString(req.Summary) > maxApprovalSummaryLength {
		writeError(w, http.StatusBadRequest, "summary must be "+strconv.Itoa(maxApprovalSummaryLength)+" characters or fewer")
		return
	}
	if utf8.RuneCountInString(req.Plan) > maxApprovalPlanLength {
		writeError(w, http.StatusBadRequest, "plan must be "+strconv.Itoa(maxApprovalPlanLength)+" characters or fewer")
		return
	}

	// Actor identity decides whose approval this is. An agent's own id comes from
	// the task token the auth middleware verified, never from the body — otherwise
	// one agent could file requests in another's name and collect its approvals.
	actorType, actorID := h.resolveActor(r, userID, workspaceID)
	agentIDValue := req.AgentID
	if actorType == "agent" {
		agentIDValue = actorID
	}
	if strings.TrimSpace(agentIDValue) == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
		return
	}
	agentUUID, ok := parseUUIDOrBadRequest(w, agentIDValue, "agent_id")
	if !ok {
		return
	}
	agent, err := h.Queries.GetAgentInWorkspace(r.Context(), db.GetAgentInWorkspaceParams{
		ID:          agentUUID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, "agent not found in this workspace")
		return
	}

	params := db.CreateAgentApprovalRequestParams{
		WorkspaceID: wsUUID,
		AgentID:     agent.ID,
		RiskClass:   req.RiskClass,
		Summary:     req.Summary,
		Plan:        req.Plan,
	}
	// The task is provenance, and only a task token can prove which one is running.
	if taskID := strings.TrimSpace(r.Header.Get("X-Task-ID")); taskID != "" && actorType == "agent" {
		if taskUUID, err := util.ParseUUID(taskID); err == nil {
			params.TaskID = taskUUID
		}
	}
	if issueID := strings.TrimSpace(req.IssueID); issueID != "" {
		issue, ok := h.loadIssueForUser(w, r, issueID)
		if !ok {
			return
		}
		if uuidToString(issue.WorkspaceID) != workspaceID {
			writeError(w, http.StatusBadRequest, "issue not found in this workspace")
			return
		}
		params.IssueID = issue.ID
	}

	created, err := h.Queries.CreateAgentApprovalRequest(r.Context(), params)
	if err != nil {
		slog.Warn("create agent approval failed", append(logger.RequestAttrs(r), "error", err, "agent_id", uuidToString(agent.ID))...)
		writeError(w, http.StatusInternalServerError, "failed to file the approval request")
		return
	}
	h.recordApprovalActivity(r, created, approvalActivityRequested, actorType, actorID, map[string]any{
		"risk_class": created.RiskClass,
		"summary":    created.Summary,
	})
	writeJSON(w, http.StatusCreated, agentApprovalToResponse(created))
}

// ListAgentApprovals returns the workspace's approval queue, newest first.
func (h *Handler) ListAgentApprovals(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	if _, ok := h.workspaceMember(w, r, workspaceID); !ok {
		return
	}
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}

	status := strings.TrimSpace(r.URL.Query().Get("status"))
	statusFilter := pgtype.Text{}
	if status != "" {
		if !isKnownApprovalStatus(status) {
			writeError(w, http.StatusBadRequest, "status must be pending, approved, rejected, executed, or cancelled")
			return
		}
		statusFilter = pgtype.Text{String: status, Valid: true}
	}
	limit := approvalPageSize(r.URL.Query().Get("limit"))

	// An agent actor sees only its own requests. It has no business reading what
	// other agents asked for, and the queue is a human review surface.
	if actorType, actorID := h.resolveActor(r, requestUserID(r), workspaceID); actorType == "agent" {
		agentUUID, err := util.ParseUUID(actorID)
		if err != nil {
			writeError(w, http.StatusForbidden, "agent identity could not be resolved")
			return
		}
		rows, err := h.Queries.ListAgentApprovalRequestsByAgent(r.Context(), db.ListAgentApprovalRequestsByAgentParams{
			AgentID: agentUUID,
			Status:  statusFilter,
			Limit:   limit,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list approval requests")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"approvals": agentApprovalsToResponse(rows)})
		return
	}

	rows, err := h.Queries.ListAgentApprovalRequests(r.Context(), db.ListAgentApprovalRequestsParams{
		WorkspaceID: wsUUID,
		Status:      statusFilter,
		Limit:       limit,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list approval requests")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"approvals": agentApprovalsToResponse(rows)})
}

func agentApprovalsToResponse(rows []db.AgentApprovalRequest) []AgentApprovalResponse {
	out := make([]AgentApprovalResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, agentApprovalToResponse(row))
	}
	return out
}

// GetAgentApproval returns one request. This is the read an agent performs to find
// out whether a person decided yet.
func (h *Handler) GetAgentApproval(w http.ResponseWriter, r *http.Request) {
	approval, ok := h.loadApprovalInWorkspace(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, agentApprovalToResponse(approval))
}

func isKnownApprovalStatus(status string) bool {
	switch status {
	case "pending", "approved", "rejected", "executed", "cancelled":
		return true
	default:
		return false
	}
}

func approvalPageSize(raw string) int32 {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return defaultApprovalPageSize
	}
	if value > maxApprovalPageSize {
		return maxApprovalPageSize
	}
	return int32(value)
}

// loadApprovalInWorkspace resolves the {approvalId} path param, confirms the caller
// is a member of its workspace, and — for an agent actor — that the request is its
// own.
func (h *Handler) loadApprovalInWorkspace(w http.ResponseWriter, r *http.Request) (db.AgentApprovalRequest, bool) {
	approvalUUID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "approvalId"), "approval id")
	if !ok {
		return db.AgentApprovalRequest{}, false
	}
	approval, err := h.Queries.GetAgentApprovalRequest(r.Context(), approvalUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "approval request not found")
		} else {
			writeError(w, http.StatusInternalServerError, "failed to load the approval request")
		}
		return db.AgentApprovalRequest{}, false
	}
	workspaceID := uuidToString(approval.WorkspaceID)
	if _, ok := h.workspaceMember(w, r, workspaceID); !ok {
		return db.AgentApprovalRequest{}, false
	}
	// Cross-workspace safety: the id is a UUID a caller could have learned
	// elsewhere, and membership was just checked against the row's own workspace, so
	// also confirm it is the workspace this request is scoped to.
	if requested := h.resolveWorkspaceID(r); requested != "" && requested != workspaceID {
		writeError(w, http.StatusNotFound, "approval request not found")
		return db.AgentApprovalRequest{}, false
	}
	if actorType, actorID := h.resolveActor(r, requestUserID(r), workspaceID); actorType == "agent" &&
		actorID != uuidToString(approval.AgentID) {
		writeError(w, http.StatusForbidden, "an agent may only read its own approval requests")
		return db.AgentApprovalRequest{}, false
	}
	return approval, true
}

// DecideAgentApprovalRequestBody is a human's decision.
type DecideAgentApprovalRequestBody struct {
	// Decision is "approve" or "reject". Spelled as a verb rather than reusing the
	// status vocabulary so a client cannot post "executed" here.
	Decision string `json:"decision"`
	Note     string `json:"note"`
}

// DecideAgentApproval records a person's decision on one request.
//
// Human-only, twice over: the route rejects machine credentials and so does this
// handler. That redundancy is the point — this is the single check that makes the
// whole mechanism worth having, and it must not depend on one line of router
// wiring staying correct.
func (h *Handler) DecideAgentApproval(w http.ResponseWriter, r *http.Request) {
	if isMachineCredentialActor(r) {
		writeError(w, http.StatusForbidden, "only a person can decide an approval request")
		return
	}
	approval, ok := h.loadApprovalInWorkspace(w, r)
	if !ok {
		return
	}
	// The approver must be able to manage the agent whose action this is: its owner,
	// or a workspace owner/admin. A member who cannot configure the agent has no
	// standing to authorize it operating production.
	agent, err := h.Queries.GetAgent(r.Context(), approval.AgentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load the requesting agent")
		return
	}
	if !h.canManageAgent(w, r, agent) {
		return
	}

	var req DecideAgentApprovalRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var status string
	switch strings.TrimSpace(req.Decision) {
	case "approve":
		status = "approved"
	case "reject":
		status = "rejected"
	default:
		writeError(w, http.StatusBadRequest, "decision must be approve or reject")
		return
	}
	if utf8.RuneCountInString(req.Note) > maxApprovalNoteLength {
		writeError(w, http.StatusBadRequest, "note must be "+strconv.Itoa(maxApprovalNoteLength)+" characters or fewer")
		return
	}

	decided, err := h.Queries.DecideAgentApprovalRequest(r.Context(), db.DecideAgentApprovalRequestParams{
		ID:           approval.ID,
		Status:       status,
		DecidedBy:    parseUUID(requestUserID(r)),
		DecisionNote: strings.TrimSpace(req.Note),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// The UPDATE's `status = 'pending'` predicate matched nothing: someone
			// decided first, or the agent cancelled. Report the conflict rather than
			// overwriting a decision already on the record.
			writeError(w, http.StatusConflict, "this request is no longer pending")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to record the decision")
		return
	}
	h.recordApprovalActivity(r, decided, approvalActivityDecided, "member", requestUserID(r), map[string]any{
		"risk_class": decided.RiskClass,
		"decision":   status,
		"note":       decided.DecisionNote,
	})
	writeJSON(w, http.StatusOK, agentApprovalToResponse(decided))
}

// RecordAgentApprovalExecutionBody reports what actually happened.
type RecordAgentApprovalExecutionBody struct {
	Note string `json:"note"`
}

// RecordAgentApprovalExecution closes the loop on an approved action.
//
// Three gates, all of which have to hold:
//
//  1. the caller is the agent the approval was granted to (or a person who can
//     manage it, recording on its behalf);
//  2. that agent's autonomy level is at least Operator — a Contributor holding an
//     approval still may not operate production, which is the "Contributor cannot
//     touch production" rule in enforceable form;
//  3. the request is in 'approved' state, enforced by the UPDATE's own predicate so
//     a rejected or already-executed request cannot be walked forward by a retry.
func (h *Handler) RecordAgentApprovalExecution(w http.ResponseWriter, r *http.Request) {
	approval, ok := h.loadApprovalInWorkspace(w, r)
	if !ok {
		return
	}
	workspaceID := uuidToString(approval.WorkspaceID)
	actorType, actorID := h.resolveActor(r, requestUserID(r), workspaceID)

	agent, err := h.Queries.GetAgent(r.Context(), approval.AgentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load the requesting agent")
		return
	}
	if actorType == "agent" {
		// loadApprovalInWorkspace already rejected another agent's request; this is
		// the same rule stated where the write happens.
		if actorID != uuidToString(approval.AgentID) {
			writeError(w, http.StatusForbidden, "an agent may only record its own approved action")
			return
		}
	} else if !h.canManageAgent(w, r, agent) {
		return
	}

	if !service.AutonomyAtLeast(agent.AutonomyLevel, service.AutonomyOperator) {
		writeError(w, http.StatusForbidden, autonomyDenialMessage(
			agent.AutonomyLevel, service.AutonomyOperator, "carry out a high-risk operation",
		))
		return
	}

	var req RecordAgentApprovalExecutionBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if utf8.RuneCountInString(req.Note) > maxApprovalNoteLength {
		writeError(w, http.StatusBadRequest, "note must be "+strconv.Itoa(maxApprovalNoteLength)+" characters or fewer")
		return
	}

	executed, err := h.Queries.MarkAgentApprovalRequestExecuted(r.Context(), db.MarkAgentApprovalRequestExecutedParams{
		ID:            approval.ID,
		ExecutionNote: strings.TrimSpace(req.Note),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusConflict, "this request is not approved, so no execution can be recorded against it")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to record the execution")
		return
	}
	h.recordApprovalActivity(r, executed, approvalActivityExecuted, actorType, actorID, map[string]any{
		"risk_class": executed.RiskClass,
		"note":       executed.ExecutionNote,
	})
	writeJSON(w, http.StatusOK, agentApprovalToResponse(executed))
}

// CancelAgentApproval withdraws a request that is no longer wanted. Either the
// agent that filed it (its plan changed) or a person who can manage that agent.
func (h *Handler) CancelAgentApproval(w http.ResponseWriter, r *http.Request) {
	approval, ok := h.loadApprovalInWorkspace(w, r)
	if !ok {
		return
	}
	actorType, actorID := h.resolveActor(r, requestUserID(r), uuidToString(approval.WorkspaceID))
	if actorType == "agent" {
		if actorID != uuidToString(approval.AgentID) {
			writeError(w, http.StatusForbidden, "an agent may only cancel its own approval requests")
			return
		}
	} else {
		agent, err := h.Queries.GetAgent(r.Context(), approval.AgentID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load the requesting agent")
			return
		}
		if !h.canManageAgent(w, r, agent) {
			return
		}
	}

	cancelled, err := h.Queries.CancelAgentApprovalRequest(r.Context(), approval.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusConflict, "this request cannot be cancelled in its current state")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to cancel the approval request")
		return
	}
	writeJSON(w, http.StatusOK, agentApprovalToResponse(cancelled))
}

// recordApprovalActivity writes the workspace-timeline half of the audit trail.
//
// Best-effort on purpose, and the one place in this file where that is the right
// call: the approval row itself already carries who decided what and when, so a
// failed timeline write loses discoverability, not the record. Contrast
// agent_env.go, where the activity row IS the only audit and a failure there
// refuses the operation.
func (h *Handler) recordApprovalActivity(
	r *http.Request,
	approval db.AgentApprovalRequest,
	action, actorType, actorID string,
	extra map[string]any,
) {
	details := map[string]any{
		"approval_id": uuidToString(approval.ID),
		"agent_id":    uuidToString(approval.AgentID),
		"status":      approval.Status,
	}
	for key, value := range extra {
		details[key] = value
	}
	encoded, err := json.Marshal(details)
	if err != nil {
		return
	}
	actor := pgtype.Text{}
	if actorType != "" {
		actor = pgtype.Text{String: actorType, Valid: true}
	}
	actorUUID := pgtype.UUID{}
	if parsed, err := util.ParseUUID(actorID); err == nil {
		actorUUID = parsed
	}
	if _, err := h.Queries.CreateActivity(r.Context(), db.CreateActivityParams{
		ID:          dbid.NewV7(),
		WorkspaceID: approval.WorkspaceID,
		IssueID:     approval.IssueID,
		ActorType:   actor,
		ActorID:     actorUUID,
		Action:      action,
		Details:     encoded,
	}); err != nil {
		slog.Warn("approval activity write failed",
			append(logger.RequestAttrs(r), "error", err, "approval_id", uuidToString(approval.ID), "action", action)...)
	}
}
