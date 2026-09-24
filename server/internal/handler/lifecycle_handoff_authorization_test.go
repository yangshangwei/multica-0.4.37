package handler

import (
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestCreateLifecycleHandoffReuseAuthorization(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}

	for _, lookup := range []string{"explicit", "duplicate-title"} {
		for _, assigneeType := range []string{"agent", "squad"} {
			for _, allowed := range []bool{false, true} {
				name := lookup + "/" + assigneeType + "/denied"
				if allowed {
					name = lookup + "/" + assigneeType + "/allowed"
				}
				t.Run(name, func(t *testing.T) {
					memberID := dbfx.User(t, "Lifecycle member", "lifecycle-reuse@multica.test")
					dbfx.Member(t, testWorkspaceID, memberID, "member")
					callerID := memberID
					if allowed {
						callerID = testUserID
					}
					agentID := dbfx.Agent(t, "Lifecycle private target", testRuntimeID)
					assigneeID := agentID
					if assigneeType == "squad" {
						assigneeID = dbfx.Squad(t, "Lifecycle private leader", agentID)
					}
					requestedAgentID := dbfx.Agent(t, "Lifecycle permitted requested target", testRuntimeID, testutil.Cols{"owner_id": callerID})
					sourceID := dbfx.Issue(t, "Lifecycle authorization source", testutil.Cols{"metadata": `{"kept":"source"}`})
					followUpID := dbfx.Issue(t, "Lifecycle authorization repair", testutil.Cols{
						"parent_issue_id": sourceID, "assignee_type": assigneeType, "assignee_id": assigneeID,
						"metadata": `{"kept":"follow-up"}`,
					})
					dbfx.Cleanup(t, `DELETE FROM comment WHERE issue_id = $1`, sourceID)
					dbfx.Cleanup(t, `DELETE FROM agent_task_queue WHERE issue_id = $1`, followUpID)
					body := lifecycleAuthorizationRequest()
					body["assignee_type"], body["assignee_id"] = "agent", requestedAgentID
					if lookup == "explicit" {
						body["follow_up_issue_id"] = followUpID
					} else {
						body["follow_up_title"] = "Lifecycle authorization repair"
					}
					before := readLifecycleAuthorizationState(t)
					response := testutil.Call(t, testHandler.CreateLifecycleHandoff,
						withURLParam(newRequestAs(callerID, http.MethodPost, "/api/issues/"+sourceID+"/lifecycle-handoffs", body), "id", sourceID))
					if !allowed {
						if after := readLifecycleAuthorizationState(t); after != before {
							t.Errorf("denied handoff changed issues=%t, comments=%d→%d, tasks=%d→%d", before.issues != after.issues, before.comments, after.comments, before.tasks, after.tasks)
						}
						assertDenialReason(t, response.Want(http.StatusForbidden), "you do not have permission to assign work to this "+assigneeType)
						return
					}
					var result lifecycleHandoffResponse
					response.Want(http.StatusCreated).JSON(&result)
					if !result.FollowUpReused || result.FollowUpCreated || result.FollowUpIssueID != followUpID || result.QueuedTaskID == "" || result.AuditCommentID == "" {
						t.Fatalf("authorized handoff did not reuse and enqueue existing follow-up: %+v", result)
					}
					if count := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1 AND agent_id = $2 AND status = 'queued'`, followUpID, agentID); count != 1 {
						t.Fatalf("tasks queued for the actual assignee = %d, want 1", count)
					}
					if count := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE agent_id = $1`, requestedAgentID); count != 0 {
						t.Fatalf("tasks queued for the requested replacement assignee = %d, want 0", count)
					}
					var conclusion string
					dbfx.QueryRow(t, `SELECT metadata->>'lifecycle_rca_conclusion' FROM issue WHERE id = $1`, followUpID).Scan(&conclusion)
					if conclusion != "confirmed" {
						t.Fatalf("authorized follow-up conclusion = %q, want confirmed", conclusion)
					}
				})
			}
		}
	}
}

func TestCreateLifecycleHandoffCreatePreservesAssigneeErrorStatus(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	memberID := dbfx.User(t, "Lifecycle denied creator", "lifecycle-create@multica.test")
	dbfx.Member(t, testWorkspaceID, memberID, "member")
	agentID := dbfx.Agent(t, "Lifecycle private create target", testRuntimeID)
	for _, tc := range []struct {
		name, assigneeType, message string
		status                      int
	}{
		{"forbidden", "agent", "you do not have permission to assign work to this agent", http.StatusForbidden},
		{"invalid", "unsupported", "assignee_type must be 'member', 'agent', or 'squad'", http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sourceID := dbfx.Issue(t, "Lifecycle create authorization source")
			body := lifecycleAuthorizationRequest()
			body["follow_up_title"] = "Lifecycle create authorization repair"
			body["assignee_type"], body["assignee_id"] = tc.assigneeType, agentID
			before := readLifecycleAuthorizationState(t)
			response := testutil.Call(t, testHandler.CreateLifecycleHandoff,
				withURLParam(newRequestAs(memberID, http.MethodPost, "/api/issues/"+sourceID+"/lifecycle-handoffs", body), "id", sourceID))
			if after := readLifecycleAuthorizationState(t); after != before {
				t.Errorf("rejected creation changed issues=%t, comments=%d→%d, tasks=%d→%d", before.issues != after.issues, before.comments, after.comments, before.tasks, after.tasks)
			}
			var result struct {
				Error string `json:"error"`
			}
			response.Want(tc.status).JSON(&result)
			if result.Error != tc.message {
				t.Fatalf("assignee rejection = %q, want %q", result.Error, tc.message)
			}
		})
	}
}

func TestCreateLifecycleHandoffDuplicateTitleKeepsParentScope(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	sourceID := dbfx.Issue(t, "Lifecycle duplicate source")
	otherSourceID := dbfx.Issue(t, "Lifecycle other source")
	otherChildID := dbfx.Issue(t, "Lifecycle same-title repair", testutil.Cols{"parent_issue_id": otherSourceID})
	// Direct fixture inserts do not advance the service's allocation counter.
	dbfx.Exec(t, `UPDATE workspace SET issue_counter = GREATEST(issue_counter, (SELECT max(number) FROM issue WHERE workspace_id = $1)) WHERE id = $1`, testWorkspaceID)
	body := lifecycleAuthorizationRequest()
	body["follow_up_title"] = "Lifecycle same-title repair"
	dbfx.Cleanup(t, `DELETE FROM comment WHERE issue_id = $1`, sourceID)
	dbfx.Cleanup(t, `DELETE FROM issue WHERE parent_issue_id = $1`, sourceID)
	var result lifecycleHandoffResponse
	testutil.Call(t, testHandler.CreateLifecycleHandoff,
		withURLParam(newRequest(http.MethodPost, "/api/issues/"+sourceID+"/lifecycle-handoffs", body), "id", sourceID)).Want(http.StatusCreated).JSON(&result)
	if !result.FollowUpCreated || result.FollowUpReused || result.FollowUpIssueID == otherChildID {
		t.Fatalf("same title under another parent must create a separate child: %+v", result)
	}
	var parentID string
	dbfx.QueryRow(t, `SELECT parent_issue_id::text FROM issue WHERE id = $1`, result.FollowUpIssueID).Scan(&parentID)
	if parentID != sourceID {
		t.Fatalf("follow-up parent = %q, want %q", parentID, sourceID)
	}
}

func lifecycleAuthorizationRequest() map[string]any {
	return map[string]any{
		"kind": "rca", "route": "bug-fix", "cause_state": "known",
		"reason": "the regression identifies the failure", "conclusion": "confirmed",
		"evidence":      []string{"a reproducible regression"},
		"diagnosis_ref": "diagnosis#regression", "regression_test": "TestLifecycleAuthorization",
	}
}

type lifecycleAuthorizationState struct {
	issues   string
	comments int
	tasks    int
}

func readLifecycleAuthorizationState(t *testing.T) lifecycleAuthorizationState {
	t.Helper()
	var state lifecycleAuthorizationState
	dbfx.QueryRow(t, `SELECT COALESCE(jsonb_agg(to_jsonb(i) ORDER BY i.id)::text, '[]') FROM issue i WHERE workspace_id = $1`, testWorkspaceID).Scan(&state.issues)
	state.comments = dbfx.Count(t, `SELECT count(*) FROM comment WHERE workspace_id = $1`, testWorkspaceID)
	state.tasks = dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id IN (SELECT id FROM issue WHERE workspace_id = $1)`, testWorkspaceID)
	return state
}
