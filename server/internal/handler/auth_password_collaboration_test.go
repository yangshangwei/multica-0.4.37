package handler

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func seedPasswordCollaborationFence(t *testing.T, userID string) {
	t.Helper()
	dbfx.InsertNoID(t, "collaboration_account_fence", testutil.Cols{"user_id": userID}, "user_id=$1", userID)
	dbfx.Cleanup(t, `DELETE FROM collaboration_account_invalidation WHERE user_id=$1`, userID)
}

func TestPasswordCollaborationInvalidationChangeAndOrdinaryControls(t *testing.T) {
	passwordTestSetup(t)
	username := fmt.Sprintf("collab%d", time.Now().UnixNano())
	registered := passwordRegister(t, username)
	seedPasswordCollaborationFence(t, registered.User.ID)
	var login LoginResponse
	passwordCall(t, "POST", "/auth/login", map[string]string{"username": username, "password": "correct horse battery staple"}, "", testHandler.PasswordLogin).Want(200).JSON(&login)
	passwordCall(t, "POST", "/auth/logout", nil, login.Token, testHandler.Logout).Want(200)
	passwordCall(t, "POST", "/api/cli-token", nil, login.Token, testHandler.IssueCliToken).Want(200)
	passwordCall(t, "PATCH", "/api/me", map[string]string{"name": "Changed display name"}, login.Token, testHandler.UpdateMe).Want(200)
	var pat CreatePATResponse
	passwordCall(t, "POST", "/api/tokens", map[string]any{"name": "Narrow ordinary token", "expires_in_days": 1}, login.Token, testHandler.CreatePersonalAccessToken).Want(201).JSON(&pat)
	dbfx.Cleanup(t, `DELETE FROM personal_access_token WHERE id=$1`, pat.ID)
	var renewal RenewPATResponse
	passwordCall(t, "POST", "/api/tokens/renew", nil, pat.Token, testHandler.RenewCurrentPersonalAccessToken).Want(200).JSON(&renewal)
	if !renewal.Renewed {
		t.Fatal("ordinary PAT control did not exercise renewal")
	}
	passwordCall(t, "DELETE", "/api/tokens/"+pat.ID, nil, login.Token, func(w http.ResponseWriter, r *http.Request) {
		testHandler.RevokePersonalAccessToken(w, testutil.WithURLParams(r, "id", pat.ID))
	}).Want(204)
	if dbfx.Count(t, `SELECT count(*) FROM collaboration_account_fence WHERE user_id=$1 AND epoch=1`, registered.User.ID) != 1 ||
		dbfx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE user_id=$1`, registered.User.ID) != 0 {
		t.Fatal("ordinary login/logout/mint/profile/PAT action invalidated persistent collaboration authority")
	}
	var changed LoginResponse
	passwordCall(t, "POST", "/api/me/password/change", map[string]string{
		"current_password": "correct horse battery staple", "new_password": "new correct horse battery staple",
	}, login.Token, testHandler.PasswordChange).Want(200).JSON(&changed)
	if dbfx.Count(t, `SELECT count(*) FROM collaboration_account_fence WHERE user_id=$1 AND epoch=2`, registered.User.ID) != 1 ||
		dbfx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE user_id=$1 AND account_epoch=2 AND credential_version=2 AND revocation_key='password:2' AND reconciled_at IS NULL`, registered.User.ID) != 1 {
		t.Fatal("real password change did not invalidate collaboration authority")
	}
	passwordCall(t, "GET", "/api/me", nil, login.Token, testHandler.GetMe).Want(401)
	passwordCall(t, "GET", "/api/me", nil, changed.Token, testHandler.GetMe).Want(200)
}

func TestPasswordCollaborationInvalidationLegacySetup(t *testing.T) {
	passwordTestSetup(t)
	now := time.Now()
	t.Setenv("MULTICA_PASSWORD_MIGRATION_CUTOFF", now.Add(-time.Minute).UTC().Format(time.RFC3339))
	t.Setenv("MULTICA_PASSWORD_MIGRATION_DEADLINE", now.Add(time.Hour).UTC().Format(time.RFC3339))
	userID := dbfx.User(t, "Legacy collaborator", fmt.Sprintf("legacy-collab-%d@example.invalid", now.UnixNano()))
	dbfx.Cleanup(t, `DELETE FROM user_password_credential WHERE user_id=$1`, userID)
	seedPasswordCollaborationFence(t, userID)
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": userID, "iat": now.Add(-time.Hour).Unix(), "exp": now.Add(time.Hour).Unix(),
	}).SignedString(auth.JWTSecret())
	if err != nil {
		t.Fatal(err)
	}
	passwordCall(t, "POST", "/api/me/password/setup", map[string]string{
		"username": fmt.Sprintf("migrated%d", now.UnixNano()), "password": "new collaborator password", "name": "Migrated collaborator",
	}, token, testHandler.PasswordSetup).Want(200)
	if dbfx.Count(t, `SELECT count(*) FROM collaboration_account_fence WHERE user_id=$1 AND epoch=2`, userID) != 1 ||
		dbfx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE user_id=$1 AND account_epoch=2 AND credential_version=1 AND revocation_key='password:1'`, userID) != 1 {
		t.Fatal("legacy identity migration retained pre-migration authority")
	}
}

func TestPasswordCollaborationInvalidationRejectedChangePreservesAuthority(t *testing.T) {
	passwordTestSetup(t)
	registered := passwordRegister(t, fmt.Sprintf("rejected%d", time.Now().UnixNano()))
	seedPasswordCollaborationFence(t, registered.User.ID)
	dbfx.Exec(t, `UPDATE user_password_credential SET must_change_password=true WHERE user_id=$1`, registered.User.ID)
	response := passwordCall(t, "POST", "/api/me/password/change", map[string]string{
		"current_password": "correct horse battery staple", "new_password": "correct horse battery staple",
	}, registered.Token, testHandler.PasswordChange).Want(400)
	if response.Map()["code"] != "password_unchanged" {
		t.Fatal("did not reach the rejected forced password-change path")
	}
	if dbfx.Count(t, `SELECT count(*) FROM collaboration_account_fence WHERE user_id=$1 AND epoch=1`, registered.User.ID) != 1 ||
		dbfx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE user_id=$1`, registered.User.ID) != 0 {
		t.Fatal("rejected password change invalidated collaboration authority")
	}
}
