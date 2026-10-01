package handler

import (
	"fmt"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestPasswordPreviouslyAuthenticatedSetupCannotCrossAdminDisableRestore(t *testing.T) {
	passwordTestSetup(t)
	now := time.Now()
	t.Setenv("MULTICA_PASSWORD_MIGRATION_CUTOFF", now.Add(-time.Minute).UTC().Format(time.RFC3339))
	t.Setenv("MULTICA_PASSWORD_MIGRATION_DEADLINE", now.Add(time.Hour).UTC().Format(time.RFC3339))
	// The account has been restored, but the pre-disable setup capability is
	// permanently revoked. A request already past middleware must fail as well.
	id := dbfx.User(t, "Restored legacy account", fmt.Sprintf("restored-legacy-%d@example.invalid", now.UnixNano()), testutil.Cols{"legacy_password_sessions_revoked_at": now})
	dbfx.Cleanup(t, "DELETE FROM user_password_credential WHERE user_id=$1", id)
	ctx := auth.WithPasswordSession(t.Context(), auth.PasswordSession{UserID: id, Kind: "jwt", Setup: true})
	request := testutil.JSONRequest("POST", "/api/me/password/setup", map[string]string{"username": fmt.Sprintf("revoked%d", now.UnixNano()), "password": "initial-password", "name": "Restored legacy account"})
	request.Header.Set("X-User-ID", id)
	testutil.Call(t, testHandler.PasswordSetup, request.WithContext(ctx)).Want(401)
	if n := dbfx.Count(t, "SELECT count(*) FROM user_password_credential WHERE user_id=$1", id); n != 0 {
		t.Fatal("revoked setup capability created credentials")
	}
}
