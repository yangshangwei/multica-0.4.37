package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/installation"
	"github.com/multica-ai/multica/server/internal/service"
)

func (h *Handler) InstallationBindingHint(w http.ResponseWriter, r *http.Request) {
	if !h.installationLimit(w, r) {
		return
	}
	result, err := h.managedInstallationService().BindingHint(r.Context(), chi.URLParam(r, "id"), r.URL.Query().Get("workspace_id"), r.URL.Query().Get("daemon_id"))
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) InstallationHeartbeat(w http.ResponseWriter, r *http.Request) {
	if !h.installationLimit(w, r) {
		return
	}
	var proof installation.HeartbeatProof
	if !passwordDecode(w, r, &proof) {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "installation id")
	if !ok {
		return
	}
	result, err := h.managedInstallationService().Heartbeat(r.Context(), uuidToString(id), proof)
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	writeJSON(w, 200, result)
}

func (h *Handler) managedInstallationService() *service.ManagedInstallationService {
	return service.NewManagedInstallationService(h.Queries, h.TxStarter, h.cfg.DeploymentID, h.cfg.ManagedInstallationsEnabled)
}

func (h *Handler) installationLimit(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Cache-Control", "no-store")
	s, ok := auth.PasswordSessionFromContext(r.Context())
	if !ok {
		adminError(w, r, 401, "session_invalid", "A password session is required")
		return false
	}
	return h.passwordLimit(w, r, auth.PasswordLimit{Key: "installation-user:" + s.UserID, Count: 120, Window: time.Minute}, auth.PasswordLimit{Key: "installation-ip:" + auth.PasswordClientIP(r), Count: 1200, Window: time.Minute})
}

func (h *Handler) InstallationChallenge(w http.ResponseWriter, r *http.Request) {
	if !h.installationLimit(w, r) {
		return
	}
	var p service.InstallationChallengeParams
	if !passwordDecode(w, r, &p) {
		return
	}
	if p.Purpose != "enroll" && p.Purpose != "bind" {
		adminError(w, r, 400, "invalid_installation_purpose", "Expected an enrollment or binding challenge")
		return
	}
	result, err := h.managedInstallationService().IssueChallenge(r.Context(), p)
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	writeJSON(w, 200, result)
}

func (h *Handler) DaemonInstallationChallenge(w http.ResponseWriter, r *http.Request) {
	if !h.installationLimit(w, r) {
		return
	}
	var p service.InstallationChallengeParams
	if !passwordDecode(w, r, &p) {
		return
	}
	if p.Purpose != "renew" {
		adminError(w, r, 400, "invalid_installation_purpose", "A scoped renewal challenge is required")
		return
	}
	result, err := h.managedInstallationService().IssueChallenge(r.Context(), p)
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	writeJSON(w, 200, result)
}

func (h *Handler) EnrollInstallation(w http.ResponseWriter, r *http.Request) {
	if !h.installationLimit(w, r) {
		return
	}
	var proof installation.Proof
	if !passwordDecode(w, r, &proof) {
		return
	}
	result, err := h.managedInstallationService().Enroll(r.Context(), proof)
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	writeJSON(w, 200, result)
}

func (h *Handler) BindInstallation(w http.ResponseWriter, r *http.Request) {
	h.redeemInstallationBinding(w, r, false)
}
func (h *Handler) RenewInstallationBinding(w http.ResponseWriter, r *http.Request) {
	h.redeemInstallationBinding(w, r, true)
}

func (h *Handler) redeemInstallationBinding(w http.ResponseWriter, r *http.Request, renew bool) {
	if !h.installationLimit(w, r) {
		return
	}
	var proof installation.Proof
	if !passwordDecode(w, r, &proof) {
		return
	}
	svc := h.managedInstallationService()
	var result service.InstallationBindingResult
	var err error
	if renew {
		result, err = svc.Renew(r.Context(), proof)
	} else {
		result, err = svc.Bind(r.Context(), proof)
	}
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	writeJSON(w, 200, result)
}
