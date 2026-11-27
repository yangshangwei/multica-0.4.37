package handler

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// Material claims require the registered host identity or that runtime's owner.
// Workspace membership alone cannot bootstrap the private claim challenge.
func materialDaemonCaller(r *http.Request, runtime db.AgentRuntime) bool {
	if runtime.DaemonID.String == "" || !runtime.OwnerID.Valid {
		return false
	}
	if daemon := middleware.DaemonIDFromContext(r.Context()); daemon != "" {
		return daemon == runtime.DaemonID.String && middleware.DaemonWorkspaceIDFromContext(r.Context()) == uuidToString(runtime.WorkspaceID)
	}
	switch middleware.DaemonAuthPathFromContext(r.Context()) {
	case middleware.DaemonAuthPathPAT, middleware.DaemonAuthPathCloudPAT, middleware.DaemonAuthPathJWT:
		return requestUserID(r) == uuidToString(runtime.OwnerID)
	}
	return false
}
func (h *Handler) materialDaemonTaskCaller(w http.ResponseWriter, r *http.Request, task db.AgentTaskQueue) bool {
	workspace, e := h.TaskService.ResolveTaskWorkspaceIDChecked(r.Context(), task)
	if e != nil || workspace == "" {
		writeError(w, 503, "material_authorization_unavailable")
		return false
	}
	runtime, e := h.Queries.GetAgentRuntimeForWorkspace(r.Context(), db.GetAgentRuntimeForWorkspaceParams{ID: task.RuntimeID, WorkspaceID: parseUUID(workspace)})
	if e != nil || !materialDaemonCaller(r, runtime) {
		writeError(w, 403, "material_daemon_required")
		return false
	}
	return true
}
func (h *Handler) GetDaemonMaterialOperation(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), "mat_") {
		writeError(w, 403, "material_daemon_required")
		return
	}
	task, ok := h.requireDaemonTaskAccess(w, r, chi.URLParam(r, "taskId"))
	if !ok {
		return
	}
	if !h.materialDaemonTaskCaller(w, r, task) {
		return
	}
	out, e := service.MaterialOperationForTask(r.Context(), h.Queries, task)
	if e != nil {
		materialFailure(w, e)
		return
	}
	writeJSON(w, 200, out)
}
func (h *Handler) ObserveDaemonMaterialOperation(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), "mat_") {
		writeError(w, 403, "material_daemon_required")
		return
	}
	task, ok := h.requireDaemonTaskAccess(w, r, chi.URLParam(r, "taskId"))
	if !ok {
		return
	}
	if !h.materialDaemonTaskCaller(w, r, task) {
		return
	}
	var in protocol.MaterialObservation
	if !decodeMaterial(w, r, &in) {
		return
	}
	if e := h.materialService().ObserveOperation(r.Context(), task.ID, in); e != nil {
		materialFailure(w, e)
		return
	}
	writeJSON(w, 200, map[string]bool{"accepted": true})
}
func (h *Handler) GetMaterialOperation(w http.ResponseWriter, r *http.Request) {
	token, hasToken := auth.VerifiedTaskTokenFromContext(r.Context())
	if !hasToken {
		writeError(w, 403, "material_task_required")
		return
	}
	task, e := h.Queries.GetAgentTask(r.Context(), token.TaskID)
	if e != nil {
		materialFailure(w, e)
		return
	}
	out, e := service.MaterialOperationForTask(r.Context(), h.Queries, task)
	if e != nil {
		materialFailure(w, e)
		return
	}
	if out.Execution.OperationID != chi.URLParam(r, "operationID") {
		writeError(w, 403, "material_task_scope_denied")
		return
	}
	writeJSON(w, 200, out)
}
func (h *Handler) PermitMaterialOperation(w http.ResponseWriter, r *http.Request) {
	token, hasToken := auth.VerifiedTaskTokenFromContext(r.Context())
	if !hasToken {
		writeError(w, 403, "material_task_required")
		return
	}
	task, e := h.Queries.GetAgentTask(r.Context(), token.TaskID)
	if e != nil {
		materialFailure(w, e)
		return
	}
	var in protocol.MaterialPermitInput
	if !decodeMaterial(w, r, &in) {
		return
	}
	out, e := h.materialService().OperationPermit(r.Context(), task, chi.URLParam(r, "operationID"), in)
	if e != nil {
		materialFailure(w, e)
		return
	}
	writeJSON(w, 200, out)
}
