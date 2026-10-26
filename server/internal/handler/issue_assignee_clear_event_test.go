package handler

import (
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// captureIssueUpdated runs call against a copy of testHandler with its own bus
// and returns the assignee_changed flag of every issue:updated event published.
func captureIssueUpdated(t *testing.T, call func(h *Handler)) []bool {
	t.Helper()
	h := *testHandler
	h.Bus = events.New()
	var flags []bool
	h.Bus.Subscribe(protocol.EventIssueUpdated, func(event events.Event) {
		payload, ok := event.Payload.(map[string]any)
		if !ok {
			t.Fatalf("issue:updated payload = %#v", event.Payload)
		}
		changed, _ := payload["assignee_changed"].(bool)
		flags = append(flags, changed)
	})
	call(&h)
	return flags
}

// Clearing an assignee sends explicit nulls (the picker's "Unassigned" option,
// board and swimlane moves into the no-assignee column). The write already
// honours them; the published event must report the change too, or the
// previous assignee gets no "unassigned" notice and no activity is recorded.
func TestUpdateIssue_ExplicitNullAssigneeReportsAssigneeChanged(t *testing.T) {
	for _, tc := range []struct {
		name string
		body map[string]any
		want bool
	}{
		{"clear_both", map[string]any{"assignee_type": nil, "assignee_id": nil}, true},
		{"title_only", map[string]any{"title": "Retitled, still assigned"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			issueID := dbfx.Issue(t, "Explicit null unassign", testutil.Cols{
				"assignee_type": "member", "assignee_id": testUserID,
			})
			req := testutil.WithURLParams(testutil.WithHeaders(testutil.JSONRequest("PUT", "/api/issues/"+issueID, tc.body),
				"X-User-ID", testUserID, "X-Workspace-ID", testWorkspaceID), "id", issueID)
			flags := captureIssueUpdated(t, func(h *Handler) {
				testutil.Call(t, h.UpdateIssue, req).Want(http.StatusOK)
			})
			if len(flags) != 1 || flags[0] != tc.want {
				t.Fatalf("assignee_changed flags = %v, want [%v]", flags, tc.want)
			}
		})
	}
}

func TestBatchUpdateIssues_ExplicitNullAssigneeReportsAssigneeChanged(t *testing.T) {
	first := dbfx.Issue(t, "Batch explicit null unassign A", testutil.Cols{"assignee_type": "member", "assignee_id": testUserID})
	second := dbfx.Issue(t, "Batch explicit null unassign B", testutil.Cols{"assignee_type": "member", "assignee_id": testUserID})
	req := testutil.WithHeaders(testutil.JSONRequest("POST", "/api/issues/batch-update", map[string]any{
		"issue_ids": []string{first, second},
		"updates":   map[string]any{"assignee_type": nil, "assignee_id": nil},
	}), "X-User-ID", testUserID, "X-Workspace-ID", testWorkspaceID)
	flags := captureIssueUpdated(t, func(h *Handler) {
		testutil.Call(t, h.BatchUpdateIssues, req).Want(http.StatusOK)
	})
	if len(flags) != 2 || !flags[0] || !flags[1] {
		t.Fatalf("assignee_changed flags = %v, want [true true]", flags)
	}
}
