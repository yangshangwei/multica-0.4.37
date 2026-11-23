package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func collaborationResourceRead(w http.ResponseWriter, r *http.Request) (pgtype.UUID, bool) {
	if !collaborationExecutionRead(w, r) {
		return pgtype.UUID{}, false
	}
	prepareCollaborationContentRequest(w, r)
	principal, ok := collaboration.PrincipalFromContext(r.Context())
	if !ok || !principal.Human() {
		writeJSON(w, http.StatusNotFound, map[string]string{"code": "not_found"})
		return pgtype.UUID{}, false
	}
	return parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
}

func (h *Handler) GetCollaborationResources(w http.ResponseWriter, r *http.Request) {
	issue, ok := collaborationResourceRead(w, r)
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	result, err := svc.Resources(r.Context(), issue)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) GetCollaborationParticipants(w http.ResponseWriter, r *http.Request) {
	issue, ok := collaborationResourceRead(w, r)
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	result, err := svc.Participants(r.Context(), issue)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) GetCollaborationResourceOptions(w http.ResponseWriter, r *http.Request) {
	issue, ok := collaborationResourceRead(w, r)
	if !ok {
		return
	}
	query := r.URL.Query()
	if len(query) != 1 || len(query["execution_workspace_id"]) != 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_resource_scope"})
		return
	}
	workspace, ok := parseUUIDOrBadRequest(w, query.Get("execution_workspace_id"), "execution_workspace_id")
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	result, err := svc.ResourceOptions(r.Context(), issue, workspace, func(member db.Member, agent db.Agent, runtime db.AgentRuntime, targets []db.AgentInvocationTarget) bool {
		return memberAllowedToViewAgent(agent, targets, uuidToString(member.UserID), member.Role) &&
			(canUseRuntimeForAgent(member, runtime) || roleAllowed(member.Role, "owner", "admin"))
	})
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) GetCollaborationResourceIdentity(w http.ResponseWriter, r *http.Request) {
	issue, ok := collaborationResourceRead(w, r)
	if !ok {
		return
	}
	query := r.URL.Query()
	for key, values := range query {
		if (key != "target_id" && key != "agent_id" && key != "proposal_id") || len(values) != 1 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_resource_scope"})
			return
		}
	}
	target, ok := parseUUIDOrBadRequest(w, query.Get("target_id"), "target_id")
	if !ok {
		return
	}
	agent, ok := parseUUIDOrBadRequest(w, query.Get("agent_id"), "agent_id")
	if !ok {
		return
	}
	var proposal pgtype.UUID
	if query.Has("proposal_id") {
		proposal, ok = parseUUIDOrBadRequest(w, query.Get("proposal_id"), "proposal_id")
		if !ok {
			return
		}
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	result, err := svc.ResourceIdentity(r.Context(), issue, target, agent, proposal)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) GetCollaborationResourceOperation(w http.ResponseWriter, r *http.Request) {
	issue, ok := collaborationResourceRead(w, r)
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	receipt, err := svc.GetResourceOperation(r.Context(), issue, chi.URLParam(r, "requestId"))
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	if receipt.Redacted {
		writeJSON(w, http.StatusOK, struct {
			Redacted        bool        `json:"redacted"`
			ProtocolVersion int         `json:"protocol_version"`
			IssueID         pgtype.UUID `json:"issue_id"`
			RequestID       string      `json:"request_id"`
			Action          string      `json:"action"`
			Status          int         `json:"status"`
			Code            string      `json:"code"`
		}{true, receipt.ProtocolVersion, receipt.IssueID, receipt.RequestID, receipt.Action, receipt.Status, receipt.Code})
		return
	}
	writeJSON(w, http.StatusOK, receipt)
}
