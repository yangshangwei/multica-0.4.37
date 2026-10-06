package handler

import (
	"fmt"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestIssueIterationFieldsRequireConfirmedOperation(t *testing.T) {
	for _, endpoint := range []string{"create", "update", "batch"} {
		for _, field := range []string{"current_iteration_id", "iteration_rollover_count", "CURRENT_ITERATION_ID"} {
			t.Run(endpoint+"/"+field, func(t *testing.T) {
				issueID, iterationID := iterationIssueFixture(t)
				dbfx.Exec(t, `UPDATE workspace SET issue_counter=(SELECT COALESCE(MAX(number),0) FROM issue WHERE workspace_id=$1) WHERE id=$1`, testWorkspaceID)
				body := map[string]any{field: nil, "title": "Must not silently ignore iteration intent"}
				if field == "iteration_rollover_count" {
					body[field] = 9
				}
				var response *testutil.Response
				switch endpoint {
				case "create":
					response = testutil.Call(t, testHandler.CreateIssue, newRequest("POST", "/api/issues", body))
				case "update":
					response = testutil.Call(t, testHandler.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+issueID, body), "id", issueID))
				case "batch":
					response = testutil.Call(t, testHandler.BatchUpdateIssues, newRequest("POST", "/api/issues/batch-update", map[string]any{"issue_ids": []string{issueID}, "updates": body}))
				}
				var problem map[string]any
				response.Want(428).JSON(&problem)
				if problem["code"] != "iteration_confirmation_required" {
					t.Fatalf("wrong explicit-write refusal: %v", problem)
				}
				if n := dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND title='Original commitment' AND current_iteration_id=$2 AND iteration_rollover_count=2 AND revision=1`, issueID, iterationID); n != 1 {
					t.Fatal("unconfirmed request mutated existing issue")
				}
				if n := dbfx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1 AND title=$2`, testWorkspaceID, body["title"]); n != 0 {
					t.Fatal("unconfirmed request created or changed an issue")
				}
			})
		}
	}
}

func TestIssueIterationOmittedFieldsPreserveExistingMembership(t *testing.T) {
	issueID, iterationID := iterationIssueFixture(t)
	for i, handler := range []string{"update", "batch"} {
		title := fmt.Sprintf("Compatible title %d", i)
		if handler == "update" {
			testutil.Call(t, testHandler.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+issueID, map[string]any{"title": title}), "id", issueID)).Want(200)
		} else {
			testutil.Call(t, testHandler.BatchUpdateIssues, newRequest("POST", "/api/issues/batch-update", map[string]any{"issue_ids": []string{issueID}, "updates": map[string]any{"title": title}})).Want(200)
		}
		if n := dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND current_iteration_id=$2 AND iteration_rollover_count=2 AND title=$3`, issueID, iterationID, title); n != 1 {
			t.Fatal("legacy edit lost membership or rollover")
		}
	}
}

func TestPluginIterationFieldsRequireConfirmedOperation(t *testing.T) {
	for _, field := range []string{"current_iteration_id", "iteration_rollover_count", "CURRENT_ITERATION_ID"} {
		t.Run(field, func(t *testing.T) {
			issueID, iterationID := iterationIssueFixture(t)
			installation := installPluginForAction(t, []string{"issues:read", "issues:write"})
			request := pluginActionRequest("PATCH", "/v1/issues/"+issueID, installation, map[string]any{"title": "Must not mutate", field: nil}, map[string]string{"issue_ref": issueID})
			testutil.Call(t, testHandler.PatchPluginIssue, request).Want(428)
			if n := dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND title='Original commitment' AND current_iteration_id=$2 AND revision=1`, issueID, iterationID); n != 1 {
				t.Fatal("plugin ignored iteration intent")
			}
		})
	}
}
