package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/testutil"
)

// The shared UI depends on the real authenticated route, not a test-only
// handler wiring. This regression starts as a missing-route failure.
func TestInboxSnoozeRouteMovesOwnedGroup(t *testing.T) {
	fx := testutil.New(testPool, testWorkspaceID, testUserID)
	issueID := fx.Issue(t, "Snooze route target")
	itemID := fx.Insert(t, "inbox_item", testutil.Cols{
		"workspace_id": testWorkspaceID, "recipient_type": "member",
		"recipient_id": testUserID, "issue_id": issueID,
		"type": "new_comment", "severity": "info", "title": "Snooze route target",
	})
	fx.Cleanup(t, "DELETE FROM inbox_critical_source WHERE workspace_id=$1 AND group_id=$2", testWorkspaceID, issueID)
	fx.Cleanup(t, "DELETE FROM inbox_snooze WHERE workspace_id=$1 AND group_id=$2", testWorkspaceID, issueID)
	response := authRequest(t, http.MethodPost, "/api/inbox/"+itemID+"/snooze", map[string]any{
		"snoozed_until":     time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano),
		"expected_revision": 0,
	})
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("snooze route status = %d, want 200", response.StatusCode)
	}
	var receipt struct {
		GroupID   string `json:"group_id"`
		Revision  int64  `json:"revision"`
		UndoToken string `json:"undo_token"`
	}
	readJSON(t, response, &receipt)
	if receipt.GroupID != issueID || receipt.Revision != 1 || receipt.UndoToken == "" {
		t.Fatalf("committed receipt = %+v, want group %s revision 1 and undo token", receipt, issueID)
	}
	var active, snoozed []map[string]any
	readJSON(t, authRequest(t, http.MethodGet, "/api/inbox", nil), &active)
	readJSON(t, authRequest(t, http.MethodGet, "/api/inbox/snoozed", nil), &snoozed)
	for _, item := range active {
		if item["issue_id"] == issueID {
			t.Fatal("snoozed group remains in the active inbox")
		}
	}
	var found bool
	for _, item := range snoozed {
		found = found || item["id"] == itemID
	}
	if !found {
		t.Fatal("snoozed group is missing from its server-owned list")
	}
}
