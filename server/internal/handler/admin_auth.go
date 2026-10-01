package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (h *Handler) platformAdminService() *service.PlatformAdminService {
	return service.NewPlatformAdminService(h.Queries, h.TxStarter)
}

// PlatformAdminNoStore precedes authentication, including rejected credentials
// and CSRF failures in the private administrative response cache policy.
func PlatformAdminNoStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/admin/") || strings.HasPrefix(r.URL.Path, "/api/installations/") || strings.HasPrefix(r.URL.Path, "/api/daemon/installations/") || strings.HasPrefix(r.URL.Path, "/api/daemon/installation-bindings") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func adminRequestID(r *http.Request) string {
	if id := chimw.GetReqID(r.Context()); id != "" {
		return id
	}
	return randomID()
}

func adminError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, status, map[string]string{"error": message, "code": code, "request_id": adminRequestID(r)})
}

func adminServiceError(w http.ResponseWriter, r *http.Request, err error) {
	var typed *service.PlatformAdminError
	switch {
	case errors.As(err, &typed):
		adminError(w, r, typed.Status, typed.Code, typed.Message)
	case errors.Is(err, auth.ErrPasswordSession):
		adminError(w, r, 401, "session_invalid", "The session is no longer valid")
	case errors.Is(err, pgx.ErrNoRows):
		adminError(w, r, 404, "admin_not_found", "The requested object was not found")
	default:
		adminError(w, r, 503, "admin_unavailable", "Administration is temporarily unavailable")
	}
}

func (h *Handler) requirePlatformAccess(w http.ResponseWriter, r *http.Request, write bool) (service.PlatformAdminIdentity, bool) {
	w.Header().Set("Cache-Control", "no-store")
	if !auth.PasswordMode() || !h.cfg.PlatformAdminEnabled {
		adminError(w, r, 403, "admin_mode_disabled", "Platform administration is not enabled")
		return service.PlatformAdminIdentity{}, false
	}
	identity, err := h.platformAdminService().Authorize(r.Context(), write)
	if err != nil {
		adminServiceError(w, r, err)
		return identity, false
	}
	return identity, true
}

// RequirePlatformRead is the route guard; handlers repeat the same gate so
// registering one on a different authenticated route cannot broaden access.
func (h *Handler) RequirePlatformRead(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := h.requirePlatformAccess(w, r, false); !ok {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) AdminMe(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requirePlatformAccess(w, r, false)
	if !ok {
		return
	}
	writeJSON(w, 200, map[string]any{"user_id": uuidToString(identity.UserID), "organization_id": uuidToString(identity.OrganizationID), "role": identity.Role, "allowed_actions": identity.AllowedActions, "supported": true})
}

func adminOperationResponse(op db.AdminOperation) map[string]any {
	return map[string]any{
		"id": uuidToString(op.ID), "organization_id": uuidToString(op.OrganizationID),
		"actor_kind": op.ActorKind, "actor_id": uuidToPtr(op.ActorID),
		"target_kind": op.TargetKind, "target_id": uuidToString(op.TargetID), "kind": op.Kind,
		"state": op.State, "result_code": op.ResultCode, "version": op.Version,
		"confirmation": op.Confirmation, "accepted_at": timestampToString(op.AcceptedAt),
		"applied_at": timestampToPtr(op.AppliedAt), "confirmed_at": timestampToPtr(op.ConfirmedAt),
		"ack_deadline": timestampToPtr(op.AckDeadline),
	}
}

func (h *Handler) AdminOperations(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requirePlatformAccess(w, r, false)
	if !ok {
		return
	}
	key, ok := parseUUIDOrBadRequest(w, r.URL.Query().Get("idempotency_key"), "idempotency key")
	if !ok {
		return
	}
	op, err := h.platformAdminService().FindOperationByKey(r.Context(), identity.OrganizationID, key)
	items := []map[string]any{}
	if err == nil {
		items = append(items, adminOperationResponse(op))
	} else if !errors.Is(err, pgx.ErrNoRows) {
		adminServiceError(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items, "next_cursor": nil, "as_of": time.Now().UTC().Format(time.RFC3339Nano), "scope": uuidToString(identity.OrganizationID)})
}

func (h *Handler) AdminOperation(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requirePlatformAccess(w, r, false)
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "operation id")
	if !ok {
		return
	}
	op, err := h.platformAdminService().GetOperation(r.Context(), identity.OrganizationID, id)
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	writeJSON(w, 200, adminOperationResponse(op))
}

func (h *Handler) AdminChangeRole(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requirePlatformAccess(w, r, true)
	if !ok {
		return
	}
	if !h.passwordLimit(w, r, auth.PasswordLimit{Key: "admin-ip:" + auth.PasswordClientIP(r), Count: 60, Window: time.Minute}, auth.PasswordLimit{Key: "admin-user:" + uuidToString(identity.UserID), Count: 10, Window: 5 * time.Minute}) {
		return
	}
	target, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "user id")
	if !ok {
		return
	}
	key, ok := parseUUIDOrBadRequest(w, r.Header.Get("Idempotency-Key"), "idempotency key")
	if !ok {
		return
	}
	var body struct {
		Role                json.RawMessage `json:"role"`
		ExpectedRole        json.RawMessage `json:"expected_role"`
		ExpectedAuthVersion int64           `json:"expected_auth_version"`
		Reason              string          `json:"reason"`
		Password            string          `json:"password"`
	}
	if !passwordDecode(w, r, &body) {
		return
	}
	var role, expected *string
	if len(body.Role) == 0 || len(body.ExpectedRole) == 0 || json.Unmarshal(body.Role, &role) != nil || json.Unmarshal(body.ExpectedRole, &expected) != nil {
		adminError(w, r, 400, "invalid_request", "role and expected_role must be a role or null")
		return
	}
	result, err := h.platformAdminService().ChangeRole(r.Context(), service.PlatformRoleChangeParams{
		OrganizationID: identity.OrganizationID, TargetUserID: target, IdempotencyKey: key,
		Role: role, ExpectedRole: expected, ExpectedAuthVersion: body.ExpectedAuthVersion,
		Reason: body.Reason, Password: body.Password, RequestID: adminRequestID(r),
	})
	body.Password = ""
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	if result.CredentialsRevoked {
		h.publishPasswordRevocation(r, uuidToString(result.TargetUserID), result.Revocation)
	}
	writeJSON(w, 200, adminOperationResponse(result.Operation))
}
