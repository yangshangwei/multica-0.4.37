package handler

import (
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// captureIssueUpdated runs call against a copy of testHandler with its own bus
// and returns the payload of every issue:updated event published.
func captureIssueUpdated(t *testing.T, call func(h *Handler)) []map[string]any {
	t.Helper()
	h := *testHandler
	h.Bus = events.New()
	var payloads []map[string]any
	h.Bus.Subscribe(protocol.EventIssueUpdated, func(event events.Event) {
		payload, ok := event.Payload.(map[string]any)
		if !ok {
			t.Fatalf("issue:updated payload = %#v", event.Payload)
		}
		payloads = append(payloads, payload)
	})
	call(&h)
	return payloads
}

func assigneeChangedFlag(payload map[string]any) bool {
	changed, _ := payload["assignee_changed"].(bool)
	return changed
}

// Clearing an assignee sends explicit nulls (the picker's "Unassigned" option,
// board and swimlane moves into the no-assignee column). The write already
// honours them; the published event must report the change too, or the
// previous assignee gets no "unassigned" notice and no activity is recorded.
func TestUpdateIssue_ExplicitNullAssigneeReportsAssigneeChanged(t *testing.T) {
	for _, tc := range []struct {
		name     string
		assigned bool
		body     map[string]any
		want     bool
	}{
		{"clear_both", true, map[string]any{"assignee_type": nil, "assignee_id": nil}, true},
		{"title_only", true, map[string]any{"title": "Retitled, still assigned"}, false},
		{"already_unassigned", false, map[string]any{"assignee_type": nil, "assignee_id": nil}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cols := testutil.Cols{}
			if tc.assigned {
				cols = testutil.Cols{"assignee_type": "member", "assignee_id": testUserID}
			}
			issueID := dbfx.Issue(t, "Explicit null unassign", cols)
			req := testutil.WithURLParams(testutil.WithHeaders(testutil.JSONRequest("PUT", "/api/issues/"+issueID, tc.body),
				"X-User-ID", testUserID, "X-Workspace-ID", testWorkspaceID), "id", issueID)
			payloads := captureIssueUpdated(t, func(h *Handler) {
				testutil.Call(t, h.UpdateIssue, req).Want(http.StatusOK)
			})
			if len(payloads) != 1 || assigneeChangedFlag(payloads[0]) != tc.want {
				t.Fatalf("issue:updated payloads = %v, want one with assignee_changed=%v", payloads, tc.want)
			}
		})
	}
}

// The batch toolbar's "Unassigned" takes the same path, and its event must
// name the previous assignee the way UpdateIssue does: the notification and
// activity listeners address the "unassigned" notice and the from-side by it.
func TestBatchUpdateIssues_ExplicitNullAssigneeReportsAssigneeChanged(t *testing.T) {
	first := dbfx.Issue(t, "Batch explicit null unassign A", testutil.Cols{"assignee_type": "member", "assignee_id": testUserID})
	second := dbfx.Issue(t, "Batch explicit null unassign B", testutil.Cols{"assignee_type": "member", "assignee_id": testUserID})
	req := testutil.WithHeaders(testutil.JSONRequest("POST", "/api/issues/batch-update", map[string]any{
		"issue_ids": []string{first, second},
		"updates":   map[string]any{"assignee_type": nil, "assignee_id": nil},
	}), "X-User-ID", testUserID, "X-Workspace-ID", testWorkspaceID)
	payloads := captureIssueUpdated(t, func(h *Handler) {
		testutil.Call(t, h.BatchUpdateIssues, req).Want(http.StatusOK)
	})
	if len(payloads) != 2 {
		t.Fatalf("issue:updated payloads = %d, want 2", len(payloads))
	}
	for _, payload := range payloads {
		prevType, _ := payload["prev_assignee_type"].(*string)
		prevID, _ := payload["prev_assignee_id"].(*string)
		if !assigneeChangedFlag(payload) || prevType == nil || *prevType != "member" || prevID == nil || *prevID != testUserID {
			t.Fatalf("batch payload = assignee_changed %v, prev_assignee_type %v, prev_assignee_id %v; want true, member, %s",
				payload["assignee_changed"], prevType, prevID, testUserID)
		}
	}
}
