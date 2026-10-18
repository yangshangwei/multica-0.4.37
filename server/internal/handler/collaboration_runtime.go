package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// The legacy daemon authenticator supports broad fallbacks. This boundary
// intentionally admits only an explicit MDT for the collaboration runtime.
func RequireCollaborationManagedBearer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if !strings.HasPrefix(middleware.BearerToken(r), "mdt_") {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"code": "managed_runtime_required", "error": "Managed runtime credential required"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requireCollaborationManagedSource(w http.ResponseWriter, r *http.Request) bool {
	prepareCollaborationContentRequest(w, r)
	source, ok := auth.PasswordSessionFromContext(r.Context())
	if !ok || source.Kind != "daemon_token" || source.UserID == "" || source.Version < 1 || source.BindingID == "" ||
		source.BindingEpoch < 1 || source.WorkspaceID == "" || source.DaemonID == "" || !strings.HasPrefix(middleware.BearerToken(r), "mdt_") {
		writeJSON(w, http.StatusForbidden, map[string]string{"code": "managed_runtime_required", "error": "Managed runtime identity required"})
		return false
	}
	return true
}

// Runtime JSON has a bounded outcome body plus fixed identity fields. It uses
// the same duplicate/unknown-field policy as the human collaboration API.
func decodeCollaborationRuntimeBody(w http.ResponseWriter, r *http.Request, target any) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"code": "json_required", "error": "JSON request required"})
		return false
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 96<<10))
	if err != nil {
		status := http.StatusBadRequest
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			status = http.StatusRequestEntityTooLarge
		}
		writeJSON(w, status, map[string]string{"code": "invalid_runtime_request", "error": "Invalid runtime request"})
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if iteration.ValidateObjectJSON(raw) != nil || decoder.Decode(target) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_runtime_request", "error": "Invalid runtime request"})
		return false
	}
	return true
}

func collaborationRuntimeRunID(w http.ResponseWriter, r *http.Request, bodyID *string) (pgtype.UUID, bool) {
	run, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "runId"), "run_id")
	if !ok || bodyID == nil {
		return run, ok
	}
	observed, ok := parseUUIDOrBadRequest(w, *bodyID, "identity.run_id")
	if !ok {
		return run, false
	}
	if observed != run {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "run_identity_mismatch", "error": "Run identity does not match request"})
		return run, false
	}
	return run, true
}

func (h *Handler) ListCollaborationRuntimeRuns(w http.ResponseWriter, r *http.Request) {
	if !requireCollaborationManagedSource(w, r) {
		return
	}
	runtimeID, ok := parseUUIDOrBadRequest(w, r.URL.Query().Get("runtime_id"), "runtime_id")
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	result, err := svc.ListRuntimeRuns(r.Context(), runtimeID)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) ClaimCollaborationRuntimeRun(w http.ResponseWriter, r *http.Request) {
	if !requireCollaborationManagedSource(w, r) {
		return
	}
	var body protocol.CollaborationRunClaimRequest
	if !decodeCollaborationRuntimeBody(w, r, &body) {
		return
	}
	run, ok := collaborationRuntimeRunID(w, r, nil)
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	result, err := svc.ClaimRuntimeRun(r.Context(), run, body)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) PermitCollaborationRuntimeRun(w http.ResponseWriter, r *http.Request) {
	if !requireCollaborationManagedSource(w, r) {
		return
	}
	var body protocol.CollaborationRunPermitRequest
	if !decodeCollaborationRuntimeBody(w, r, &body) {
		return
	}
	run, ok := collaborationRuntimeRunID(w, r, &body.Identity.RunID)
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	result, err := svc.PermitRuntimeRun(r.Context(), run, body)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func collaborationNarrowBearer(w http.ResponseWriter, r *http.Request, prefix string) (string, bool) {
	prepareCollaborationContentRequest(w, r)
	token := middleware.BearerToken(r)
	valid := len(token) == len(prefix)+64 && strings.HasPrefix(token, prefix)
	if valid {
		for _, digit := range token[len(prefix):] {
			if !(digit >= '0' && digit <= '9') && !(digit >= 'a' && digit <= 'f') {
				valid = false
				break
			}
		}
	}
	if !valid {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"code": "invalid_collaboration_capability", "error": "Collaboration capability required"})
		return "", false
	}
	return token, true
}

func (h *Handler) RecordCollaborationRuntimeOutcome(w http.ResponseWriter, r *http.Request) {
	token, ok := collaborationNarrowBearer(w, r, service.CollaborationBusinessTokenPrefix)
	if !ok {
		return
	}
	var body protocol.CollaborationRunOutcome
	if !decodeCollaborationRuntimeBody(w, r, &body) {
		return
	}
	run, ok := collaborationRuntimeRunID(w, r, &body.Identity.RunID)
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	if err := svc.RecordRuntimeOutcome(r.Context(), run, token, body); err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"code": "outcome_recorded"})
}

func (h *Handler) GetCollaborationRuntimeStopIntent(w http.ResponseWriter, r *http.Request) {
	token, ok := collaborationNarrowBearer(w, r, service.CollaborationTerminationTokenPrefix)
	if !ok {
		return
	}
	run, ok := collaborationRuntimeRunID(w, r, nil)
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	result, err := svc.RuntimeStopIntent(r.Context(), run, token)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) AcceptCollaborationRuntimeProof(w http.ResponseWriter, r *http.Request) {
	token, ok := collaborationNarrowBearer(w, r, service.CollaborationTerminationTokenPrefix)
	if !ok {
		return
	}
	var body protocol.CollaborationRunProof
	if !decodeCollaborationRuntimeBody(w, r, &body) {
		return
	}
	run, ok := collaborationRuntimeRunID(w, r, &body.Identity.RunID)
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	result, err := svc.AcceptRuntimeProof(r.Context(), run, token, body)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
