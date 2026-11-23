package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// A new-version reviewer cannot inherit its runtime owner's general issue-write
// authority, even when a token was minted without its required capability.
func TestTaskCapabilityReviewerCannotRewriteIssue(t *testing.T) {
	fx := testutil.New(testPool, testWorkspaceID, testUserID)
	runtimeID := fx.Runtime(t, "Capability reviewer runtime")
	agentID := fx.Agent(t, "Capability reviewer", runtimeID, testutil.Cols{
		"template_key": "code-reviewer", "template_version": 2, "autonomy_level": "observer",
	})
	issueID := fx.Issue(t, "Original title")
	taskID := fx.Task(t, agentID, testutil.Cols{"status": "running", "runtime_id": runtimeID, "issue_id": issueID})
	token, err := auth.GenerateAgentTaskToken()
	if err != nil {
		t.Fatal(err)
	}
	fx.Insert(t, "task_token", testutil.Cols{"token_hash": auth.HashToken(token), "task_id": taskID, "agent_id": agentID, "workspace_id": testWorkspaceID, "user_id": testUserID, "expires_at": time.Now().Add(time.Hour)})
	req := testutil.JSONRequest(http.MethodPut, "/api/issues/"+issueID, map[string]any{"title": "Unauthorized rewrite"})
	req.Header.Set("Authorization", "Bearer "+token)
	testutil.Call(t, testServer.Config.Handler.ServeHTTP, req).Want(http.StatusForbidden)
	var title string
	fx.QueryRow(t, `SELECT title FROM issue WHERE id=$1`, issueID).Scan(&title)
	if title != "Original title" {
		t.Fatalf("denied write changed title: %q", title)
	}
}
