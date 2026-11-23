package handler

import (
	"net/http"
	"strconv"

	"github.com/multica-ai/multica/server/internal/service"
)

func (h *Handler) adminCollaborationGateService() *service.AdminCollaborationGateService {
	return service.NewAdminCollaborationGateService(h.Queries, h.TxStarter)
}

func (h *Handler) AdminCollaborationGate(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requirePlatformAccess(w, r, false); !ok {
		return
	}
	if r.URL.RawQuery != "" {
		adminServiceError(w, r, adminQueryError())
		return
	}
	view, err := h.adminCollaborationGateService().Get(r.Context())
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (h *Handler) AdminChangeCollaborationGate(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requirePlatformAccess(w, r, true)
	if !ok {
		return
	}
	key, ok := parseUUIDOrBadRequest(w, r.Header.Get("Idempotency-Key"), "idempotency key")
	if !ok {
		return
	}
	var body struct {
		Action        string  `json:"action"`
		ExpectedEpoch *string `json:"expected_execution_epoch"`
		Reason        string  `json:"reason"`
	}
	if !passwordDecode(w, r, &body) {
		return
	}
	var epoch int64
	if body.ExpectedEpoch != nil {
		parsed, err := strconv.ParseInt(*body.ExpectedEpoch, 10, 64)
		if err != nil || parsed < 1 || strconv.FormatInt(parsed, 10) != *body.ExpectedEpoch {
			adminError(w, r, 400, "invalid_request", "A current execution epoch is required")
			return
		}
		epoch = parsed
	}
	result, err := h.adminCollaborationGateService().Change(r.Context(), service.AdminCollaborationGateParams{OrganizationID: actor.OrganizationID, IdempotencyKey: key, Action: body.Action, Reason: body.Reason, RequestID: adminRequestID(r), ExpectedExecutionEpoch: epoch})
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"operation": adminOperationResponse(result.Operation), "target": result.Gate})
}
