package handler

import (
	"net/http"

	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func (h *Handler) collaborationKnowledgeRuntimeService() *service.CollaborationService {
	s := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	s.KnowledgeContextEnabled = h.TaskService != nil && h.TaskService.KnowledgeContextEnabled
	return s
}

func (h *Handler) PrepareCollaborationRuntimeKnowledge(w http.ResponseWriter, r *http.Request) {
	if !requireCollaborationManagedSource(w, r) {
		return
	}
	var p protocol.CollaborationRunKnowledgePrepare
	if !decodeCollaborationRuntimeBody(w, r, &p) {
		return
	}
	run, ok := collaborationRuntimeRunID(w, r, &p.Identity.RunID)
	if !ok {
		return
	}
	out, err := h.collaborationKnowledgeRuntimeService().PrepareRuntimeKnowledge(r.Context(), run, auth.HashToken(middleware.BearerToken(r)), p)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) AuthorizeCollaborationRuntimeKnowledge(w http.ResponseWriter, r *http.Request) {
	if !requireCollaborationManagedSource(w, r) {
		return
	}
	var p protocol.CollaborationRunKnowledgeAuthorization
	if !decodeCollaborationRuntimeBody(w, r, &p) {
		return
	}
	run, ok := collaborationRuntimeRunID(w, r, &p.Identity.RunID)
	if !ok {
		return
	}
	out, err := h.collaborationKnowledgeRuntimeService().AuthorizeRuntimeKnowledge(r.Context(), run, auth.HashToken(middleware.BearerToken(r)), p)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) GetCollaborationRuntimeKnowledgeStatus(w http.ResponseWriter, r *http.Request) {
	token, ok := collaborationNarrowBearer(w, r, service.CollaborationTerminationTokenPrefix)
	if !ok {
		return
	}
	run, ok := collaborationRuntimeRunID(w, r, nil)
	if !ok {
		return
	}
	out, err := h.collaborationKnowledgeRuntimeService().RuntimeKnowledgeStatus(r.Context(), run, token)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
