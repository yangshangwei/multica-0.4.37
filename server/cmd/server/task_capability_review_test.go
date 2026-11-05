package main

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// This uses the production router rather than a handler-only registration.
// The claim tests own minting; this test owns every registered Auth boundary.
func TestTaskCapabilityReviewProductionRouter(t *testing.T) {
	fx := testutil.New(testPool, testWorkspaceID, testUserID)
	runtime := fx.Runtime(t, "Scoped router reviewer")
	agent := fx.Agent(t, "Scoped router reviewer", runtime, testutil.Cols{"template_key": "code-reviewer", "template_version": 2, "autonomy_level": "observer"})
	issue := fx.Issue(t, "Scoped router issue")
	outside := fx.Issue(t, "Router outside scope")
	task := fx.Task(t, agent, testutil.Cols{"status": "running", "issue_id": issue, "runtime_id": runtime, "originator_user_id": testUserID, "accountable_user_id": testUserID, "claim_generation": 1})
	c := auth.TaskCapability{Version: 1, Profile: auth.TaskCapabilityReviewer, IssueID: issue, OriginatorID: testUserID, RuntimeID: runtime, ClaimGeneration: 1}
	capability, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	token, err := auth.GenerateAgentTaskToken()
	if err != nil {
		t.Fatal(err)
	}
	fx.Insert(t, "task_token", testutil.Cols{"token_hash": auth.HashToken(token), "task_id": task, "agent_id": agent, "workspace_id": testWorkspaceID, "user_id": testUserID, "expires_at": time.Now().Add(time.Hour), "api_capability": capability})
	for _, tc := range []struct {
		method, path string
		body         any
		want         int
	}{
		{"GET", "/api/issues/" + issue, nil, 200},
		{"GET", "/api/issues/" + issue + "/comments", nil, 200},
		{"GET", "/api/issues/" + issue + "/timeline", nil, 200},
		{"POST", "/api/issues/" + issue + "/comments", map[string]any{"content": "Scoped result"}, 201},
		{"GET", "/api/issues/" + outside, nil, 403},
		{"PUT", "/api/issues/" + issue, map[string]any{"title": "Forbidden change"}, 403},
		{"POST", "/api/cli-token", nil, 403},
		{"GET", "/api/plugin-bridge/v1/installations", nil, 403},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			req := testutil.JSONRequest(tc.method, tc.path, tc.body)
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("X-Actor-Source", "member")
			testutil.Call(t, testServer.Config.Handler.ServeHTTP, req).Want(tc.want)
		})
	}
	var eligible bool
	fx.QueryRow(t, `SELECT dispatch_eligible FROM comment WHERE source_task_id=$1 AND content='Scoped result'`, task).Scan(&eligible)
	if eligible {
		t.Fatal("production router persisted an executable scoped comment")
	}
	var title string
	fx.QueryRow(t, `SELECT title FROM issue WHERE id=$1`, issue).Scan(&title)
	if title != "Scoped router issue" {
		t.Fatal("production router admitted forbidden mutation")
	}
	// Real human control remains permitted on the same resource.
	req := testutil.JSONRequest(http.MethodPut, "/api/issues/"+issue, map[string]any{"title": "Human change"})
	humanToken, err := generateTestJWT(testUserID, "reviewer@example.test", "Review control")
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+humanToken)
	req.Header.Set("X-Workspace-ID", testWorkspaceID)
	testutil.Call(t, testServer.Config.Handler.ServeHTTP, req).Want(200)
}
