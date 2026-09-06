package handler

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestBatchUpdateIssues_MergesRepeatedUpdatesObjects(t *testing.T) {
	for _, tc := range []struct {
		name    string
		updates string
	}{
		{"mixed_case", `"updates":{"assignee_id":null,"assignee_type":null},"Updates":{"title":"Clarified"}`},
		{"reversed_case", `"Updates":{"assignee_id":null,"assignee_type":null},"updates":{"title":"Clarified"}`},
		{"same_key", `"updates":{"assignee_id":null,"assignee_type":null},"updates":{"title":"Clarified"}`},
		{"empty_later_object", `"updates":{"assignee_id":null,"assignee_type":null,"title":"Clarified"},"Updates":{}`},
		{"null_later_object", `"updates":{"assignee_id":null,"assignee_type":null,"title":"Clarified"},"updates":null`},
		{"null_between_objects", `"updates":{"assignee_id":null,"assignee_type":null},"Updates":null,"UPDATES":{"title":"Clarified"}`},
		{"null_before_object", `"updates":null,"Updates":{"assignee_id":null,"assignee_type":null,"title":"Clarified"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			issueID := dbfx.Issue(t, "Batch update object merge", testutil.Cols{
				"assignee_type": "member", "assignee_id": testUserID,
			})
			body := fmt.Sprintf(`{"issue_ids":[%q],%s}`, issueID, tc.updates)
			req := testutil.WithHeaders(testutil.JSONRequest("POST", "/api/issues/batch-update", body),
				"X-User-ID", testUserID, "X-Workspace-ID", testWorkspaceID)
			var out struct {
				Updated int `json:"updated"`
			}
			testutil.Call(t, testHandler.BatchUpdateIssues, req).Want(http.StatusOK).JSON(&out)
			if out.Updated != 1 {
				t.Errorf("updated = %d, want 1", out.Updated)
			}
			var title string
			var unassigned bool
			dbfx.QueryRow(t, `SELECT title, assignee_type IS NULL AND assignee_id IS NULL FROM issue WHERE id = $1`, issueID).
				Scan(&title, &unassigned)
			if title != "Clarified" || !unassigned {
				t.Errorf("title = %q, unassigned = %v; want Clarified and true", title, unassigned)
			}
		})
	}
}

func TestBatchUpdateIssues_RepeatedUpdatesCannotHideObserverUnassignment(t *testing.T) {
	agentID, taskID := autonomyTestAgent(t, "Repeated updates observer", "observer")
	issueID := dbfx.Issue(t, "Keep this assignment", testutil.Cols{
		"assignee_type": "member", "assignee_id": testUserID,
	})
	body := fmt.Sprintf(`{"issue_ids":[%q],"updates":{"assignee_id":null,"assignee_type":null},"updates":{"title":"Changed"}}`, issueID)
	req := asAgent(testutil.WithHeaders(testutil.JSONRequest("POST", "/api/issues/batch-update", body),
		"X-User-ID", testUserID, "X-Workspace-ID", testWorkspaceID), agentID, taskID)
	testutil.Call(t, testHandler.BatchUpdateIssues, req).Want(http.StatusForbidden)
	var title, assigneeID string
	dbfx.QueryRow(t, `SELECT title, assignee_id::text FROM issue WHERE id = $1`, issueID).Scan(&title, &assigneeID)
	if title != "Keep this assignment" || assigneeID != testUserID {
		t.Errorf("denied request changed issue: title = %q, assignee = %q", title, assigneeID)
	}
}
