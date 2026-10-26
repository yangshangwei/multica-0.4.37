package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
)

func (h *Handler) adminCollaborationService() *service.AdminCollaborationService {
	return service.NewAdminCollaborationService(h.Queries, h.TxStarter, h.cfg.PlatformAdminEnabled)
}

func adminCollaborationPageQuery(w http.ResponseWriter, r *http.Request) (service.AdminCollaborationPageQuery, bool) {
	out := service.AdminCollaborationPageQuery{Limit: 50}
	if value := r.URL.Query().Get("limit"); value != "" {
		limit, err := strconv.Atoi(value)
		if err != nil || limit < 1 || limit > 100 {
			adminError(w, r, 400, "invalid_page_limit", "The page limit must be between 1 and 100")
			return out, false
		}
		out.Limit = limit
	}
	if value := r.URL.Query().Get("cursor"); value != "" {
		cursor, ok := parseUUIDOrBadRequest(w, value, "cursor")
		if !ok {
			return out, false
		}
		out.Cursor = cursor
	}
	return out, true
}

func (h *Handler) AdminCollaborationIssues(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requirePlatformAccess(w, r, true)
	if !ok {
		return
	}
	query, ok := adminCollaborationPageQuery(w, r)
	if !ok {
		return
	}
	page, err := h.adminCollaborationService().ListIssues(r.Context(), actor.OrganizationID, query)
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h *Handler) AdminCollaborationIssue(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requirePlatformAccess(w, r, true)
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	detail, err := h.adminCollaborationService().GetIssue(r.Context(), actor.OrganizationID, id)
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (h *Handler) AdminCollaborationReopenCandidates(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requirePlatformAccess(w, r, true)
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	query, ok := adminCollaborationPageQuery(w, r)
	if !ok {
		return
	}
	page, err := h.adminCollaborationService().ReopenCandidates(r.Context(), actor.OrganizationID, id, query)
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func adminCollaborationResultResponse(result service.AdminCollaborationResult) map[string]any {
	if result.MemoryResult != nil {
		return map[string]any{"operation": adminOperationResponse(result.Operation), "result": result.MemoryResult}
	}
	return map[string]any{"operation": adminOperationResponse(result.Operation), "result": result.Result}
}

// AdminCollaborationMemoryVersions lists shared-context history metadata only.
func (h *Handler) AdminCollaborationMemoryVersions(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requirePlatformAccess(w, r, true)
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	query := service.CollaborationMemoryVersionsQuery{Limit: 50}
	if value := r.URL.Query().Get("limit"); value != "" {
		limit, err := strconv.Atoi(value)
		if err != nil || limit < 1 || limit > 100 {
			adminError(w, r, 400, "invalid_page_limit", "The page limit must be between 1 and 100")
			return
		}
		query.Limit = limit
	}
	if value := r.URL.Query().Get("cursor"); value != "" {
		version, valid := collaborationPositiveVersion(value)
		if !valid {
			adminError(w, r, 400, "invalid_cursor", "The page cursor is invalid")
			return
		}
		query.BeforeVersion = pgtype.Int8{Int64: version, Valid: true}
	}
	page, err := h.adminCollaborationService().ListMemoryVersions(r.Context(), actor.OrganizationID, id, query)
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h *Handler) AdminCollaborationMemoryPurge(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requirePlatformAccess(w, r, true)
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	version, valid := collaborationPositiveVersion(chi.URLParam(r, "version"))
	if !valid {
		adminError(w, r, 400, "invalid_version", "An exact positive shared-context version is required")
		return
	}
	key, ok := parseUUIDOrBadRequest(w, r.Header.Get("Idempotency-Key"), "idempotency key")
	if !ok {
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if !decodeCollaborationWorkflowBodyLimit(w, r, &body, 8192) {
		return
	}
	result, err := h.adminCollaborationService().PurgeMemoryVersion(r.Context(), service.AdminCollaborationMemoryPurgeParams{
		OrganizationID: actor.OrganizationID, IssueID: id, IdempotencyKey: key, Version: version, Reason: body.Reason, RequestID: adminRequestID(r)})
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, adminCollaborationResultResponse(result))
}

func (h *Handler) AdminCollaborationLifecycle(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requirePlatformAccess(w, r, true)
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	key, ok := parseUUIDOrBadRequest(w, r.Header.Get("Idempotency-Key"), "idempotency key")
	if !ok {
		return
	}
	var body struct {
		Action                        string          `json:"action"`
		ExpectedCollaborationRevision string          `json:"expected_collaboration_revision"`
		ExpectedStatus                string          `json:"expected_status"`
		TargetStatus                  string          `json:"target_status"`
		StatusOptionsVersion          string          `json:"status_options_version"`
		Reason                        string          `json:"reason"`
		SuccessorUserID               json.RawMessage `json:"successor_user_id"`
	}
	if !decodeCollaborationWorkflowBodyLimit(w, r, &body, 4096) {
		return
	}
	revision, ok := collaborationPositiveVersion(body.ExpectedCollaborationRevision)
	if !ok {
		adminError(w, r, 400, "invalid_version", "An exact positive collaboration revision is required")
		return
	}
	p := service.AdminCollaborationLifecycleParams{OrganizationID: actor.OrganizationID, IssueID: id, IdempotencyKey: key,
		ExpectedCollaborationRevision: revision, ExpectedStatus: body.ExpectedStatus, TargetStatus: body.TargetStatus,
		Action: body.Action, StatusOptionsVersion: body.StatusOptionsVersion, Reason: body.Reason, RequestID: adminRequestID(r)}
	if len(body.SuccessorUserID) != 0 {
		var value string
		if json.Unmarshal(body.SuccessorUserID, &value) != nil || value == "" {
			adminError(w, r, 400, "invalid_successor", "An explicit successor must be a participant UUID")
			return
		}
		successor, valid := parseUUIDOrBadRequest(w, value, "successor_user_id")
		if !valid {
			return
		}
		p.SuccessorUserID = &successor
	}
	result, err := h.adminCollaborationService().ChangeLifecycle(r.Context(), p)
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	if result.ParentWakeup != nil {
		h.notifyParentOfChildDone(r.Context(), result.ParentWakeup.Before, result.ParentWakeup.After)
	}
	writeJSON(w, http.StatusOK, adminCollaborationResultResponse(result))
}

func (h *Handler) AdminCollaborationOperation(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requirePlatformAccess(w, r, true)
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	key, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "requestId"), "request_id")
	if !ok {
		return
	}
	result, err := h.adminCollaborationService().GetOperation(r.Context(), actor.OrganizationID, id, key)
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, adminCollaborationResultResponse(result))
}
