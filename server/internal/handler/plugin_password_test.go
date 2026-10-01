package handler

import (
	"context"
	"fmt"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"net/http"
	"testing"
	"time"
)

func TestPasswordCallbackRevocationPreservesPluginActor(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "password")
	withPluginsV1Flag(t, testHandler, true)
	cleanupPluginInstallations(t)
	withCallbackTokens(t)
	installationID := installHookPlugin(t)
	issueID := dbfx.Issue(t, "Password callback revocation")
	dbfx.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": testUserID, "username": fmt.Sprintf("callback%d", time.Now().UnixNano()), "password_hash": "unused-test-hash", "session_version": int64(1)}, "user_id=$1", testUserID)
	installation, err := testHandler.PluginService.InstallationForWorkspace(context.Background(), parseUUID(testWorkspaceID), installationID)
	if err != nil {
		t.Fatal(err)
	}
	ctx := auth.WithPasswordSession(context.Background(), auth.PasswordSession{UserID: testUserID, Version: 1, Kind: "jwt"})
	token, err := testHandler.PluginService.Callbacks.Issue(ctx, service.HookInvocation{Installation: installation, Actor: service.HookActor{Type: "member", ID: parseUUID(testUserID)}})
	if err != nil {
		t.Fatal(err)
	}
	call := func(token string) *testutil.Response {
		return testutil.Call(t, testHandler.GetPluginIssue, callbackRequest(token, http.MethodGet, "/v1/issues/"+issueID, nil, map[string]string{"issue_ref": issueID}))
	}
	call(token).Want(http.StatusOK)
	pluginToken := issueCallbackToken(t, installationID, service.HookActor{Type: "plugin", ID: parseUUID(installationID)})
	dbfx.Exec(t, "UPDATE user_password_credential SET session_version=2 WHERE user_id=$1", testUserID)
	call(token).Want(http.StatusForbidden)
	call(pluginToken).Want(http.StatusOK)
}
