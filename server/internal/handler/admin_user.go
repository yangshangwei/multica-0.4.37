package handler

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type adminUserResponse struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Username       *string  `json:"username"`
	PlatformRole   *string  `json:"platform_role"`
	Status         string   `json:"status"`
	AuthVersion    int64    `json:"auth_version"`
	WorkspaceCount int64    `json:"workspace_count"`
	CreatedAt      string   `json:"created_at"`
	AllowedActions []string `json:"allowed_actions"`
}

func adminUserDTO(actor service.PlatformAdminIdentity, id pgtype.UUID, name string, email, username, role pgtype.Text, created, disabled pgtype.Timestamptz, version pgtype.Int8, change pgtype.Bool, count int64) adminUserResponse {
	status := "active"
	blocked := auth.IsTemporarilyDisabledUser(uuidToString(id), email.String)
	switch {
	case disabled.Valid || blocked:
		status = "disabled"
	case !version.Valid:
		status = "setup_required"
	case change.Bool:
		status = "password_change_required"
	}
	actions := []string{}
	if actor.Role == service.PlatformRoleSuperAdmin {
		if actor.UserID != id && !blocked {
			actions = append(actions, "recover-password")
			if disabled.Valid {
				actions = append(actions, "restore")
			} else {
				actions = append(actions, "disable")
			}
		}
		if role.Valid || status == "active" && version.Valid {
			actions = append(actions, "role")
		}
	}
	return adminUserResponse{ID: uuidToString(id), Name: name, Username: textToPtr(username), PlatformRole: textToPtr(role), Status: status, AuthVersion: version.Int64, WorkspaceCount: count, CreatedAt: timestampToString(created), AllowedActions: actions}
}
func adminOptionalTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: !value.IsZero()}
}

func (h *Handler) AdminUsers(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requirePlatformAccess(w, r, false)
	if !ok {
		return
	}
	p, err := ParseAdminListQuery(r, AdminListScope{Resource: "users", ActorID: uuidToString(actor.UserID), OrganizationID: uuidToString(actor.OrganizationID)}, []string{"q", "status", "role"}, time.Now())
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	status, role := p.Filters["status"], p.Filters["role"]
	if len(p.Filters["q"]) > 128 || strings.ContainsRune(p.Filters["q"], 0) || status != "" && !adminContains([]string{"active", "disabled", "setup_required", "password_change_required"}, status) || role != "" && !adminContains([]string{"administrators", "super_admin", "platform_observer"}, role) {
		adminServiceError(w, r, adminQueryError())
		return
	}
	args := db.ListPlatformUsersParams{OrganizationID: actor.OrganizationID, AsOf: adminOptionalTime(p.AsOf), CreatedFrom: adminOptionalTime(p.From), CreatedTo: adminOptionalTime(p.To), Search: p.Filters["q"], RoleFilter: role, BeforeCreated: adminOptionalTime(p.AfterTime), RowLimit: 100}
	if p.AfterID != "" {
		args.BeforeID = parseUUID(p.AfterID)
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	items := []adminUserResponse{}
	var next *string
	quality := "complete"
	var lastIncludedTime time.Time
	var lastIncludedID string
	// State includes the shared emergency denylist. Scan bounded SQL pages so
	// filtering never duplicates that list in SQL or silently hides later rows.
	for scanned := 0; scanned < 1000; {
		rows, queryErr := h.Queries.ListPlatformUsers(ctx, args)
		if queryErr != nil {
			adminServiceError(w, r, queryErr)
			return
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			scanned++
			args.BeforeCreated = row.CreatedAt
			args.BeforeID = row.ID
			item := adminUserDTO(actor, row.ID, row.Name, row.Email, row.Username, row.Role, row.CreatedAt, row.DisabledAt, row.SessionVersion, row.MustChangePassword, row.WorkspaceCount)
			if status != "" && item.Status != status {
				continue
			}
			if len(items) == int(p.Limit) {
				cursor, e := p.Cursor(lastIncludedTime, lastIncludedID)
				if e != nil {
					adminServiceError(w, r, e)
					return
				}
				next = &cursor
				break
			}
			items = append(items, item)
			lastIncludedTime = row.CreatedAt.Time
			lastIncludedID = item.ID
		}
		if next != nil || len(rows) < int(args.RowLimit) {
			break
		}
		if scanned >= 1000 {
			cursor, e := p.Cursor(args.BeforeCreated.Time, uuidToString(args.BeforeID))
			if e != nil {
				adminServiceError(w, r, e)
				return
			}
			next = &cursor
			quality = "partial"
		}
	}
	writeJSON(w, 200, map[string]any{"items": items, "next_cursor": next, "as_of": p.AsOf.Format(time.RFC3339Nano), "scope": uuidToString(actor.OrganizationID), "data_quality": quality, "registration": map[string]bool{"enabled": h.cfg.AllowSignup, "approval_required": false}})
}
func (h *Handler) AdminUser(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requirePlatformAccess(w, r, false)
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "user id")
	if !ok {
		return
	}
	row, err := h.Queries.GetPlatformUserDetail(r.Context(), db.GetPlatformUserDetailParams{OrganizationID: actor.OrganizationID, UserID: id})
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	memberships, err := h.Queries.ListPlatformUserMemberships(r.Context(), db.ListPlatformUserMembershipsParams{UserID: row.ID, OrganizationID: actor.OrganizationID})
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	item := adminUserDTO(actor, row.ID, row.Name, row.Email, row.Username, row.Role, row.CreatedAt, row.DisabledAt, row.SessionVersion, row.MustChangePassword, row.WorkspaceCount)
	writeJSON(w, 200, map[string]any{"user": item, "memberships": memberships, "memberships_truncated": row.WorkspaceCount > 100, "scope": uuidToString(actor.OrganizationID)})
}
func (h *Handler) AdminDisableUser(w http.ResponseWriter, r *http.Request) {
	h.adminChangeAccount(w, r, "disable")
}
func (h *Handler) AdminRestoreUser(w http.ResponseWriter, r *http.Request) {
	h.adminChangeAccount(w, r, "restore")
}
func (h *Handler) AdminRecoverPassword(w http.ResponseWriter, r *http.Request) {
	h.adminChangeAccount(w, r, "recover-password")
}
func (h *Handler) adminChangeAccount(w http.ResponseWriter, r *http.Request, action string) {
	actor, ok := h.requirePlatformAccess(w, r, true)
	if !ok {
		return
	}
	if !h.passwordLimit(w, r, auth.PasswordLimit{Key: "admin-ip:" + auth.PasswordClientIP(r), Count: 60, Window: time.Minute}, auth.PasswordLimit{Key: "admin-user:" + uuidToString(actor.UserID), Count: 10, Window: 5 * time.Minute}) {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "user id")
	if !ok {
		return
	}
	key, ok := parseUUIDOrBadRequest(w, r.Header.Get("Idempotency-Key"), "idempotency key")
	if !ok {
		return
	}
	var body struct {
		ExpectedAuthVersion *int64 `json:"expected_auth_version"`
		Reason              string `json:"reason"`
		Password            string `json:"password"`
		TemporaryPassword   string `json:"temporary_password"`
		Username            string `json:"username"`
	}
	if !passwordDecode(w, r, &body) {
		return
	}
	if body.ExpectedAuthVersion == nil || action != "recover-password" && (body.TemporaryPassword != "" || body.Username != "") {
		adminError(w, r, 400, "invalid_request", "Expected account version is required and recovery fields apply only to password recovery")
		return
	}
	result, err := h.platformAdminService().ChangeAccount(r.Context(), service.PlatformAccountChangeParams{OrganizationID: actor.OrganizationID, TargetUserID: id, IdempotencyKey: key, Action: action, ExpectedAuthVersion: *body.ExpectedAuthVersion, Reason: body.Reason, Password: body.Password, TemporaryPassword: body.TemporaryPassword, Username: body.Username, RequestID: adminRequestID(r)})
	body.Password = ""
	body.TemporaryPassword = ""
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	if result.CredentialsRevoked {
		h.publishPasswordRevocation(r, uuidToString(result.TargetUserID), result.Revocation)
	}
	writeJSON(w, 200, adminOperationResponse(result.Operation))
}
