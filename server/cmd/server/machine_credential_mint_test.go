package main

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// TestMachineCredentialTaskTokenCannotMintHumanCredentials closes the
// credential-laundering chain the 2026-09-06 audit demonstrated: a mat_ task
// token carries its owner's user id, so without a human-actor gate it can mint
// a JWT on /api/cli-token or a personal access token on /api/tokens and then
// present that clean human credential wherever the task token is refused
// (self-promotion, approval decisions). Every request traverses NewRouter's
// real authentication and actor guards; tokens never leave this test.
func TestMachineCredentialTaskTokenCannotMintHumanCredentials(t *testing.T) {
	if testPool == nil || testServer == nil {
		t.Fatal("isolated real-router DB fixture is required")
	}

	base := testutil.New(testPool, testWorkspaceID, testUserID)
	workspaceID := base.Workspace(t, "Machine credential gate", fmt.Sprintf("machine-credential-gate-%d", time.Now().UnixNano()))
	base.Member(t, workspaceID, testUserID, "member")
	fx := testutil.New(testPool, workspaceID, testUserID)
	runtimeID := fx.Runtime(t, "Machine credential gate runtime")
	agentID := fx.Agent(t, "Machine credential gate observer", runtimeID, testutil.Cols{"autonomy_level": "observer"})
	taskID := fx.Task(t, agentID, testutil.Cols{"status": "running", "runtime_id": runtimeID})

	taskToken, err := auth.GenerateAgentTaskToken()
	if err != nil {
		t.Fatalf("generate task token: %v", err)
	}
	fx.Insert(t, "task_token", testutil.Cols{
		"token_hash":   auth.HashToken(taskToken),
		"task_id":      taskID,
		"agent_id":     agentID,
		"workspace_id": workspaceID,
		"user_id":      testUserID,
		"expires_at":   time.Now().Add(time.Hour),
	})

	// A live human PAT the task actor will try to revoke, so the DELETE case
	// targets a real row instead of the idempotent no-row 204.
	humanPAT, err := auth.GeneratePATToken()
	if err != nil {
		t.Fatalf("generate human PAT: %v", err)
	}
	patID := fx.Insert(t, "personal_access_token", testutil.Cols{
		"user_id":      testUserID,
		"name":         "Machine credential gate human PAT",
		"token_hash":   auth.HashToken(humanPAT),
		"token_prefix": humanPAT[:12],
		"expires_at":   time.Now().Add(24 * time.Hour),
	})

	// call takes the testing.T of the (sub)test doing the calling, so a
	// failed Want reports against that case instead of the parent.
	call := func(t *testing.T, token, method, path string, body any) *testutil.Response {
		t.Helper()
		req := testutil.JSONRequest(method, path, body)
		req.Header.Set("Authorization", "Bearer "+token)
		return testutil.Call(t, testServer.Config.Handler.ServeHTTP, req)
	}

	patsBefore := fx.Count(t,
		`SELECT count(*) FROM personal_access_token WHERE user_id = $1 AND revoked = false`, testUserID)

	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"cli_token_jwt", http.MethodPost, "/api/cli-token", nil},
		{"create_pat", http.MethodPost, "/api/tokens", map[string]any{"name": "task token minted PAT"}},
		{"list_pats", http.MethodGet, "/api/tokens", nil},
		{"renew_pat", http.MethodPost, "/api/tokens/current/renew", nil},
		{"revoke_pat", http.MethodDelete, "/api/tokens/" + patID, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			call(t, taskToken, tc.method, tc.path, tc.body).Want(http.StatusForbidden)
		})
	}

	// Nothing was minted, none of the owner's tokens were revoked, and the
	// laundering target — the agent's autonomy level — is untouched even via
	// the direct self-promotion attempt.
	if patsAfter := fx.Count(t,
		`SELECT count(*) FROM personal_access_token WHERE user_id = $1 AND revoked = false`,
		testUserID); patsAfter != patsBefore {
		t.Fatalf("the task actor changed the user's live PAT rows: before=%d after=%d", patsBefore, patsAfter)
	}
	call(t, taskToken, http.MethodPut, "/api/agents/"+agentID,
		map[string]any{"autonomy_level": "operator"}).Want(http.StatusForbidden)
	var level string
	fx.QueryRow(t, `SELECT autonomy_level FROM agent WHERE id = $1`, agentID).Scan(&level)
	if level != "observer" {
		t.Fatalf("agent autonomy level changed: %s", level)
	}
}

// TestMachineCredentialHumanCallerStillMints is the control: the gate must not
// break the flows it exists to protect — the browser JWT → CLI token exchange
// and a human creating a personal access token.
func TestMachineCredentialHumanCallerStillMints(t *testing.T) {
	if testPool == nil || testServer == nil {
		t.Fatal("isolated real-router DB fixture is required")
	}
	fx := testutil.New(testPool, testWorkspaceID, testUserID)

	call := func(method, path string, body any) *testutil.Response {
		t.Helper()
		req := testutil.JSONRequest(method, path, body)
		req.Header.Set("Authorization", "Bearer "+testToken)
		return testutil.Call(t, testServer.Config.Handler.ServeHTTP, req)
	}

	var cliToken struct {
		Token string `json:"token"`
	}
	call(http.MethodPost, "/api/cli-token", nil).Want(http.StatusOK).JSON(&cliToken)
	if cliToken.Token == "" {
		t.Fatal("cli-token returned no JWT")
	}

	var minted struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	call(http.MethodPost, "/api/tokens", map[string]any{"name": "human control PAT"}).
		Want(http.StatusCreated).JSON(&minted)
	fx.Cleanup(t, `DELETE FROM personal_access_token WHERE id = $1`, minted.ID)
	if minted.Token == "" {
		t.Fatal("token creation returned no raw PAT")
	}
}
