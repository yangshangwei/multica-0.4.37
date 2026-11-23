package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
)

// Waiting cancellation preserves its narrow scope: a claimed input is refused,
// never converted to Stop or a request to reorder the surrounding queue.
func (h *Handler) CancelCollaborationWaitingCommand(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	var body struct {
		collaborationResourceRequestBody
		TargetID                      string `json:"target_id"`
		ExpectedCollaborationRevision string `json:"expected_collaboration_revision"`
	}
	if !decodeCollaborationWorkflowBody(w, r, &body) {
		return
	}
	request, ok := collaborationResourceRequest(w, r, body.collaborationResourceRequestBody)
	if !ok {
		return
	}
	command, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "commandId"), "command_id")
	if !ok {
		return
	}
	target, ok := parseUUIDOrBadRequest(w, body.TargetID, "target_id")
	if !ok {
		return
	}
	revision, ok := collaborationPositiveVersion(body.ExpectedCollaborationRevision)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_version"})
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	receipt, err := svc.CancelWaitingCommand(r.Context(), service.CollaborationCancelCommandParams{CollaborationQueueRequest: request, TargetID: target, CommandID: command, ExpectedCollaborationRevision: revision})
	writeCollaborationResourceResponse(w, receipt, err)
}

func (h *Handler) GetCollaborationCommandOperation(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	issue, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	receipt, err := svc.GetCommandOperation(r.Context(), issue, chi.URLParam(r, "requestId"))
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
