package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/service"
)

func (h *Handler) AdminChangeAdmission(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requirePlatformAccess(w, r, true)
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "installation id")
	if !ok {
		return
	}
	key, ok := parseUUIDOrBadRequest(w, r.Header.Get("Idempotency-Key"), "idempotency key")
	if !ok {
		return
	}
	var body struct {
		Admission       string `json:"admission"`
		ExpectedVersion string `json:"expected_admission_version"`
		Reason          string `json:"reason"`
	}
	if !passwordDecode(w, r, &body) {
		return
	}
	version, err := strconv.ParseInt(body.ExpectedVersion, 10, 64)
	if err != nil || version < 1 || strconv.FormatInt(version, 10) != body.ExpectedVersion {
		adminError(w, r, 400, "invalid_admission_version", "A current admission version is required")
		return
	}
	result, err := h.managedInstallationService().ChangeAdmission(r.Context(), service.AdminAdmissionParams{OrganizationID: actor.OrganizationID, InstallationID: id, IdempotencyKey: key, ExpectedVersion: version, Admission: body.Admission, Reason: body.Reason, RequestID: adminRequestID(r)}, h.TaskService)
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"operation": adminOperationResponse(result.Operation), "target": map[string]any{"id": uuidToString(result.Installation.ID), "admission": result.Installation.Admission, "admission_version": strconv.FormatInt(result.Installation.AdmissionVersion, 10)}})
}
