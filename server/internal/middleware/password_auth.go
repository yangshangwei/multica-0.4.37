package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/auth"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func passwordAuthError(w http.ResponseWriter, err error) {
	status, code := 503, "auth_unavailable"
	if errors.Is(err, auth.ErrPasswordSession) || errors.Is(err, pgx.ErrNoRows) {
		status, code = 401, "session_revoked"
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": http.StatusText(status), "code": code})
}
func passwordAuthenticate(w http.ResponseWriter, r *http.Request, q *db.Queries) (auth.PasswordSession, bool) {
	var s auth.PasswordSession
	raw, cookie := extractToken(r)
	if raw == "" {
		passwordAuthError(w, auth.ErrPasswordSession)
		return s, false
	}
	if cookie && !auth.ValidateCSRF(r) {
		writeError(w, 403, "CSRF validation failed")
		return s, false
	}
	if q == nil {
		passwordAuthError(w, errors.New("authentication database unavailable"))
		return s, false
	}
	switch {
	case strings.HasPrefix(raw, "mcn_"), strings.HasPrefix(raw, "mdt_"), strings.HasPrefix(raw, "mpi_"), strings.HasPrefix(raw, "mpc_"):
		passwordAuthError(w, auth.ErrPasswordSession)
		return s, false
	case strings.HasPrefix(raw, "mul_"):
		token, err := q.GetPersonalAccessTokenByHash(r.Context(), auth.HashToken(raw))
		if err != nil {
			passwordAuthError(w, err)
			return s, false
		}
		s = auth.PasswordSession{UserID: uuidToString(token.UserID), Version: token.AuthVersion, Kind: "pat"}
	case strings.HasPrefix(raw, "mat_"):
		token, err := q.GetTaskTokenByHash(r.Context(), auth.HashToken(raw))
		if err != nil {
			passwordAuthError(w, err)
			return s, false
		}
		s, err = auth.PasswordSessionForTask(r.Context(), q, token)
		if err != nil {
			passwordAuthError(w, err)
			return s, false
		}
		r.Header.Set("X-Actor-Source", "task_token")
		r.Header.Set("X-Agent-ID", uuidToString(token.AgentID))
		r.Header.Set("X-Task-ID", uuidToString(token.TaskID))
		r.Header.Set("X-Workspace-ID", uuidToString(token.WorkspaceID))
	default:
		token, err := jwt.Parse(raw, func(token *jwt.Token) (any, error) { return auth.JWTSecret(), nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithJSONNumber())
		if err != nil || !token.Valid {
			passwordAuthError(w, auth.ErrPasswordSession)
			return s, false
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			passwordAuthError(w, auth.ErrPasswordSession)
			return s, false
		}
		s, err = auth.CheckPasswordJWT(r.Context(), q, claims)
		if err != nil {
			passwordAuthError(w, err)
			return s, false
		}
	}
	if s.Kind != "jwt" {
		if _, err := auth.CheckPasswordVersion(auth.WithPasswordSession(r.Context(), s), q, s.UserID, s.Version); err != nil {
			passwordAuthError(w, err)
			return s, false
		}
	}
	if !s.Allows(r.Method, r.URL.Path) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(403)
		code := "password_change_required"
		if s.Setup {
			code = "account_setup_required"
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Complete account setup before continuing", "code": code})
		return s, false
	}
	if rejectTemporarilyDisabledUser(w, r, s.UserID, "", s.Kind) {
		return s, false
	}
	r.Header.Set("X-User-ID", s.UserID)
	r.Header.Del("X-User-Email")
	return s, true
}
