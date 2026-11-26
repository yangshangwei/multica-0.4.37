package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// materialTaskGuard is a ceiling tied to one persisted plan run. It does not
// expand built-in Reporter/Reviewer capabilities or trust a request header.
func (h *Handler) materialTaskGuard(w http.ResponseWriter, r *http.Request, token db.TaskToken) (bool, bool) {
	task, err := h.Queries.GetAgentTask(r.Context(), token.TaskID)
	if err != nil {
		writeError(w, 403, "material_plan_task_denied")
		return true, false
	}
	if !task.IssueID.Valid && task.TriggerEvidenceKind.String != "material_baseline" {
		return false, true
	}
	plan, err := h.Queries.GetMaterialPlanForIssue(r.Context(), task.IssueID)
	if errors.Is(err, pgx.ErrNoRows) && task.TriggerEvidenceKind.String != "material_baseline" {
		return false, true
	}
	if err != nil {
		writeError(w, 503, "material_task_authorization_unavailable")
		return true, false
	}
	if token.AgentID != plan.AgentID || token.WorkspaceID != plan.WorkspaceID || len(token.ApiCapability) > 0 {
		writeError(w, 403, "material_plan_task_denied")
		return true, false
	}
	if err = service.AuthorizeMaterialPlanTask(r.Context(), h.Queries, task); err != nil {
		writeError(w, 403, "material_plan_task_denied")
		return true, false
	}
	if r.URL.RawPath != "" && r.URL.RawPath != r.URL.Path || r.URL.RawQuery != "" || r.URL.ForceQuery {
		writeError(w, 403, "material_plan_task_denied")
		return true, false
	}
	projectPath := "/api/projects/" + util.UUIDToString(plan.ProjectID)
	materialPath := projectPath + "/materials/" + util.UUIDToString(plan.MaterialID)
	requestPath := strings.TrimSuffix(r.URL.Path, "/")
	allowed := r.Method == http.MethodGet && (requestPath == materialPath || requestPath == projectPath+"/resources" || requestPath == "/api/issues/"+util.UUIDToString(plan.IssueID)) || r.Method == http.MethodPost && requestPath == materialPath+"/revisions"
	if !allowed {
		writeError(w, 403, "material_plan_task_denied")
		return true, false
	}
	return true, true
}
