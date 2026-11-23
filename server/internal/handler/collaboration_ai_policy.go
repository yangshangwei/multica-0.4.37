package handler

import (
	"net/http"

	"github.com/multica-ai/multica/server/internal/service"
)

type collaborationAIPolicyBody struct {
	collaborationWorkflowBody
	AIPolicy string `json:"ai_policy"`
}

func (h *Handler) UpdateCollaborationAIPolicy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	r.Header.Del("X-Workspace-ID")
	r.Header.Del("X-Workspace-Slug")
	var body collaborationAIPolicyBody
	if !decodeCollaborationWorkflowBody(w, r, &body) {
		return
	}
	request, ok := collaborationWorkflowRequest(w, r, body.collaborationWorkflowBody)
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	receipt, err := svc.ChangeAIPolicy(r.Context(), service.CollaborationAIPolicyParams{CollaborationWorkflowRequest: request, AIPolicy: body.AIPolicy})
	writeCollaborationWorkflowResponse(w, receipt, err)
}
