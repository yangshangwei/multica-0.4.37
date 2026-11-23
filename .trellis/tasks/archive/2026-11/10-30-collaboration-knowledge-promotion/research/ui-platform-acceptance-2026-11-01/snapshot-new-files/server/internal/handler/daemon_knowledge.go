package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func writeTaskKnowledgeError(w http.ResponseWriter, err error) {
	var domain *service.CollaborationError
	if errors.As(err, &domain) {
		writeError(w, domain.Status, domain.Code)
		return
	}
	writeError(w, http.StatusServiceUnavailable, "knowledge_unavailable")
}

func decodeTaskKnowledgeRequest[T any](w http.ResponseWriter, r *http.Request, out *T) bool {
	r.Body = http.MaxBytesReader(w, r.Body, protocol.CollaborationKnowledgeMaxDeliveryBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(out) != nil || decoder.Decode(new(any)) != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid_knowledge_request")
		return false
	}
	return true
}

func (h *Handler) taskKnowledgeCaller(w http.ResponseWriter, r *http.Request) (service.TaskKnowledgeCaller, db.AgentTaskQueue, bool) {
	// A human PAT/JWT fallback accepted by older daemon endpoints does not
	// become a machine preparation credential. Keep task/runtime scope checks.
	if middleware.DaemonAuthPathFromContext(r.Context()) != middleware.DaemonAuthPathDaemonToken {
		writeError(w, http.StatusForbidden, "knowledge_authorization_denied")
		return service.TaskKnowledgeCaller{}, db.AgentTaskQueue{}, false
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !strings.HasPrefix(token, "mdt_") {
		writeError(w, http.StatusForbidden, "knowledge_authorization_denied")
		return service.TaskKnowledgeCaller{}, db.AgentTaskQueue{}, false
	}
	task, ok := h.requireDaemonTaskAccess(w, r, chi.URLParam(r, "taskId"))
	if !ok {
		return service.TaskKnowledgeCaller{}, task, false
	}
	return service.TaskKnowledgeCaller{RuntimeID: task.RuntimeID, Kind: "daemon_token", TokenHash: auth.HashToken(token)}, task, true
}

func (h *Handler) PrepareTaskKnowledge(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	caller, task, ok := h.taskKnowledgeCaller(w, r)
	if !ok {
		return
	}
	var req protocol.TaskKnowledgePrepareRequest
	if !decodeTaskKnowledgeRequest(w, r, &req) {
		return
	}
	out, err := h.TaskService.PrepareTaskKnowledge(r.Context(), caller, task.ID, req)
	if err != nil {
		writeTaskKnowledgeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) AuthorizeTaskKnowledgeLaunch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	caller, task, ok := h.taskKnowledgeCaller(w, r)
	if !ok {
		return
	}
	var req protocol.TaskKnowledgeAuthorizeRequest
	if !decodeTaskKnowledgeRequest(w, r, &req) {
		return
	}
	out, err := h.TaskService.AuthorizeTaskKnowledgeLaunch(r.Context(), caller, task.ID, req)
	if err != nil {
		writeTaskKnowledgeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) ObserveTaskKnowledgeLaunch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	caller, task, ok := h.taskKnowledgeCaller(w, r)
	if !ok {
		return
	}
	var req protocol.TaskKnowledgeObservation
	if !decodeTaskKnowledgeRequest(w, r, &req) {
		return
	}
	if err := h.TaskService.ObserveTaskKnowledgeLaunch(r.Context(), caller, task.ID, req); err != nil {
		writeTaskKnowledgeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"accepted": true})
}
