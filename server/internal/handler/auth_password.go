package handler

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/auth"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func passwordError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": message, "code": code})
}

func (h *Handler) passwordAvailable(w http.ResponseWriter) bool {
	if !auth.PasswordMode() {
		passwordError(w, 403, "auth_mode_disabled", "Password authentication is disabled")
		return false
	}
	return true
}

func (h *Handler) passwordLimit(w http.ResponseWriter, r *http.Request, limits ...auth.PasswordLimit) bool {
	wait, err := h.PasswordLimiter.Allow(r.Context(), limits...)
	if err != nil {
		w.Header().Set("Retry-After", "1")
		passwordError(w, 503, "auth_unavailable", "Authentication service unavailable")
		return false
	}
	if wait > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(max(1, int(math.Ceil(wait.Seconds())))))
		passwordError(w, 429, "rate_limited", "Too many attempts. Try again later")
		return false
	}
	return true
}

func passwordDecode(w http.ResponseWriter, r *http.Request, target any) bool {
	// JSON-only requests require a browser preflight; cross-site HTML forms
	// cannot mint a session through CORS's simple-request path.
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		passwordError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		passwordError(w, 400, "invalid_request", "Invalid request body")
		return false
	}
	if err := d.Decode(new(any)); err != io.EOF {
		passwordError(w, 400, "invalid_request", "Invalid request body")
		return false
	}
	return true
}

func passwordKDFError(w http.ResponseWriter, err error) {
	w.Header().Set("Retry-After", "1")
	passwordError(w, 503, "auth_unavailable", "Authentication service unavailable")
}

func passwordDBError(w http.ResponseWriter, err error) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		passwordError(w, 409, "username_taken", "Username is already taken")
		return
	}
	passwordError(w, 503, "auth_unavailable", "Authentication service unavailable")
}

func (h *Handler) passwordSessionResponse(w http.ResponseWriter, r *http.Request, status int, user db.User, c db.UserPasswordCredential) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": uuidToString(user.ID), "email": user.Email.String, "name": user.Name, "auth_version": c.SessionVersion, "iat": time.Now().Unix(), "exp": time.Now().Add(auth.AuthTokenTTL()).Unix()})
	signed, err := token.SignedString(auth.JWTSecret())
	if err != nil {
		passwordDBError(w, err)
		return
	}
	if err = auth.SetAuthCookies(w, signed); err != nil {
		passwordDBError(w, err)
		return
	}
	// Wildcard CDN cookies cannot participate in per-user version revocation.
	auth.ClearCloudFrontCookies(w)
	response := h.userToResponse(user)
	response.Username = c.Username
	response.RequiresPasswordChange = c.MustChangePassword
	writeJSON(w, status, LoginResponse{Token: signed, User: response})
}

type passwordAccountRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

func validatePasswordAccount(w http.ResponseWriter, req *passwordAccountRequest) bool {
	var err error
	req.Username, err = auth.NormalizeUsername(req.Username)
	if err != nil {
		passwordError(w, 400, "invalid_username", err.Error())
		return false
	}
	req.Name, err = auth.ValidatePasswordName(req.Name)
	if err != nil {
		passwordError(w, 400, "invalid_name", err.Error())
		return false
	}
	if err = auth.ValidatePassword(req.Password); err != nil {
		passwordError(w, 400, "invalid_password", err.Error())
		return false
	}
	return true
}

func (h *Handler) PasswordRegister(w http.ResponseWriter, r *http.Request) {
	if !h.passwordAvailable(w) {
		return
	}
	if !h.passwordLimit(w, r, auth.PasswordLimit{Key: "register-ip:" + auth.PasswordClientIP(r), Count: 30, Window: 5 * time.Minute}) {
		return
	}
	if !h.cfg.AllowSignup {
		passwordError(w, 403, "signup_disabled", "Registration is disabled")
		return
	}
	var req passwordAccountRequest
	if !passwordDecode(w, r, &req) || !validatePasswordAccount(w, &req) {
		return
	}
	hash, err := auth.HashPassword(r.Context(), req.Password)
	if err != nil {
		passwordKDFError(w, err)
		return
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		passwordDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	user, err := q.CreateUser(r.Context(), db.CreateUserParams{Name: req.Name})
	if err != nil {
		passwordDBError(w, err)
		return
	}
	credential, err := q.CreatePasswordCredential(r.Context(), db.CreatePasswordCredentialParams{UserID: user.ID, Username: req.Username, PasswordHash: hash})
	if err != nil {
		passwordDBError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		passwordDBError(w, err)
		return
	}
	h.passwordSessionResponse(w, r, 201, user, credential)
}

func (h *Handler) PasswordLogin(w http.ResponseWriter, r *http.Request) {
	if !h.passwordAvailable(w) {
		return
	}
	if !h.passwordLimit(w, r, auth.PasswordLimit{Key: "login-ip:" + auth.PasswordClientIP(r), Count: 300, Window: time.Minute}) {
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !passwordDecode(w, r, &req) {
		return
	}
	username, err := auth.NormalizeUsername(req.Username)
	if err != nil || auth.ValidatePassword(req.Password) != nil {
		passwordError(w, 400, "invalid_request", "Invalid username or password format")
		return
	}
	if !h.passwordLimit(w, r, auth.PasswordLimit{Key: "login-account:" + username, Count: 10, Window: 5 * time.Minute}) {
		return
	}
	credential, err := h.Queries.GetPasswordCredentialByUsername(r.Context(), username)
	if errors.Is(err, pgx.ErrNoRows) {
		if err = auth.DummyPasswordVerification(r.Context(), req.Password); err != nil {
			passwordKDFError(w, err)
			return
		}
		passwordError(w, 401, "invalid_credentials", "Invalid username or password")
		return
	}
	if err != nil {
		passwordDBError(w, err)
		return
	}
	valid, err := auth.VerifyPassword(r.Context(), credential.PasswordHash, req.Password)
	if err != nil {
		passwordKDFError(w, err)
		return
	}
	if !valid {
		passwordError(w, 401, "invalid_credentials", "Invalid username or password")
		return
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		passwordDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	if _, err = q.LockPasswordUser(r.Context(), credential.UserID); err != nil {
		passwordDBError(w, err)
		return
	}
	current, err := q.GetPasswordCredential(r.Context(), credential.UserID)
	if err != nil {
		passwordDBError(w, err)
		return
	}
	if current.SessionVersion != credential.SessionVersion || current.PasswordHash != credential.PasswordHash {
		passwordError(w, 401, "invalid_credentials", "Invalid username or password")
		return
	}
	user, err := q.GetUser(r.Context(), credential.UserID)
	if err != nil {
		passwordDBError(w, err)
		return
	}
	if user.DisabledAt.Valid || auth.IsTemporarilyDisabledUser(uuidToString(user.ID), user.Email.String) {
		passwordError(w, 403, "account_disabled", auth.TemporarilyDisabledUserError)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		passwordDBError(w, err)
		return
	}
	h.passwordSessionResponse(w, r, 200, user, current)
}

func (h *Handler) passwordEditSession(w http.ResponseWriter, r *http.Request) (auth.PasswordSession, bool) {
	if !h.passwordAvailable(w) {
		return auth.PasswordSession{}, false
	}
	session, ok := auth.PasswordSessionFromContext(r.Context())
	if !ok || session.Kind != "jwt" {
		passwordError(w, 403, "session_required", "A user login session is required")
		return session, false
	}
	if !h.passwordLimit(w, r, auth.PasswordLimit{Key: "edit-ip:" + auth.PasswordClientIP(r), Count: 60, Window: time.Minute}, auth.PasswordLimit{Key: "edit-user:" + session.UserID, Count: 10, Window: 5 * time.Minute}) {
		return session, false
	}
	return session, true
}

func (h *Handler) PasswordSetup(w http.ResponseWriter, r *http.Request) {
	session, ok := h.passwordEditSession(w, r)
	if !ok {
		return
	}
	if !session.Setup {
		passwordError(w, 409, "already_configured", "Account is already configured")
		return
	}
	var req passwordAccountRequest
	if !passwordDecode(w, r, &req) || !validatePasswordAccount(w, &req) {
		return
	}
	hash, err := auth.HashPassword(r.Context(), req.Password)
	if err != nil {
		passwordKDFError(w, err)
		return
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		passwordDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	uid := parseUUID(session.UserID)
	if _, err = q.LockPasswordUser(r.Context(), uid); err != nil {
		passwordDBError(w, err)
		return
	}
	user, err := q.GetUser(r.Context(), uid)
	if err != nil {
		passwordSessionError(w, err)
		return
	}
	if user.DisabledAt.Valid || user.LegacyPasswordSessionsRevokedAt.Valid {
		passwordSessionError(w, auth.ErrPasswordSession)
		return
	}
	if _, err = q.GetPasswordCredential(r.Context(), uid); err == nil {
		passwordError(w, 409, "already_configured", "Account is already configured")
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		passwordDBError(w, err)
		return
	}
	_, deadline, err := auth.PasswordMigrationWindow()
	if err != nil || !time.Now().Before(deadline) {
		passwordError(w, 401, "migration_expired", "Account migration window has expired")
		return
	}
	credential, err := q.CreatePasswordCredential(r.Context(), db.CreatePasswordCredentialParams{UserID: uid, Username: req.Username, PasswordHash: hash})
	if err != nil {
		passwordDBError(w, err)
		return
	}
	user, err = q.UpdateUser(r.Context(), db.UpdateUserParams{ID: uid, Name: req.Name})
	if err != nil {
		passwordDBError(w, err)
		return
	}
	revocation, err := auth.RevokePasswordCredentials(r.Context(), tx, uid)
	if err != nil {
		passwordDBError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		passwordDBError(w, err)
		return
	}
	h.publishPasswordRevocation(r, session.UserID, revocation)
	h.passwordSessionResponse(w, r, 200, user, credential)
}

func (h *Handler) PasswordChange(w http.ResponseWriter, r *http.Request) {
	session, ok := h.passwordEditSession(w, r)
	if !ok {
		return
	}
	if session.Setup {
		passwordError(w, 403, "account_setup_required", "Complete account setup first")
		return
	}
	var req struct {
		Current string `json:"current_password"`
		New     string `json:"new_password"`
	}
	if !passwordDecode(w, r, &req) {
		return
	}
	if auth.ValidatePassword(req.Current) != nil || auth.ValidatePassword(req.New) != nil {
		passwordError(w, 400, "invalid_password", "Password must contain 6–128 characters")
		return
	}
	uid := parseUUID(session.UserID)
	credential, err := h.Queries.GetPasswordCredential(r.Context(), uid)
	if err != nil {
		passwordDBError(w, err)
		return
	}
	valid, err := auth.VerifyPassword(r.Context(), credential.PasswordHash, req.Current)
	if err != nil {
		passwordKDFError(w, err)
		return
	}
	if !valid {
		passwordError(w, 401, "invalid_credentials", "Current password is incorrect")
		return
	}
	hash, err := auth.HashPassword(r.Context(), req.New)
	if err != nil {
		passwordKDFError(w, err)
		return
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		passwordDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	if _, err = q.LockPasswordUser(r.Context(), uid); err != nil {
		passwordDBError(w, err)
		return
	}
	user, err := q.GetUser(r.Context(), uid)
	if err != nil {
		passwordSessionError(w, err)
		return
	}
	if user.DisabledAt.Valid {
		passwordSessionError(w, auth.ErrPasswordSession)
		return
	}
	current, err := q.GetPasswordCredential(r.Context(), uid)
	if err != nil {
		passwordDBError(w, err)
		return
	}
	if current.SessionVersion != session.Version || current.PasswordHash != credential.PasswordHash {
		passwordError(w, 401, "session_revoked", "Session is no longer valid")
		return
	}
	current, err = q.ChangePasswordCredential(r.Context(), db.ChangePasswordCredentialParams{UserID: uid, PasswordHash: hash, SessionVersion: session.Version})
	if err != nil {
		passwordDBError(w, err)
		return
	}
	revocation, err := auth.RevokePasswordCredentials(r.Context(), tx, uid)
	if err != nil {
		passwordDBError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		passwordDBError(w, err)
		return
	}
	h.publishPasswordRevocation(r, session.UserID, revocation)
	h.passwordSessionResponse(w, r, 200, user, current)
}

func (h *Handler) issuePasswordCLIToken(w http.ResponseWriter, r *http.Request) {
	source, ok := auth.PasswordSessionFromContext(r.Context())
	if !ok || source.Kind != "jwt" {
		passwordError(w, 403, "session_required", "A user login session is required")
		return
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		passwordDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	session, err := auth.LockPasswordSession(r.Context(), q)
	if err != nil {
		passwordSessionError(w, err)
		return
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": session.UserID, "auth_version": session.Version, "iat": time.Now().Unix(), "exp": time.Now().Add(auth.AuthTokenTTL()).Unix()})
	signed, err := token.SignedString(auth.JWTSecret())
	if err != nil {
		passwordDBError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		passwordDBError(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"token": signed})
}

func (h *Handler) publishPasswordRevocation(r *http.Request, targetUserID string, result auth.PasswordRevocation) {
	if h.Hub != nil {
		h.Hub.DisconnectUser(targetUserID)
	}
	if h.DaemonHub != nil {
		h.DaemonHub.DisconnectUser(targetUserID)
	}
	if h.TaskService != nil {
		for _, task := range result.CancelledTasks {
			ws := h.TaskService.ResolveTaskWorkspaceID(r.Context(), task)
			if ws != "" {
				h.TaskService.BroadcastCancelledTasks(r.Context(), ws, []db.AgentTaskQueue{task})
			}
		}
	}
	seen := make(map[string]bool)
	for _, rt := range result.OfflineRuntimes {
		ws := uuidToString(rt.WorkspaceID)
		if !seen[ws] {
			seen[ws] = true
			h.publish(protocol.EventDaemonRegister, ws, "member", r.Header.Get("X-User-ID"), map[string]any{"action": "revoke"})
		}
	}
}

func passwordSessionError(w http.ResponseWriter, err error) {
	if errors.Is(err, auth.ErrPasswordSession) || errors.Is(err, pgx.ErrNoRows) {
		passwordError(w, 401, "session_revoked", "Session is no longer valid")
		return
	}
	passwordDBError(w, err)
}
