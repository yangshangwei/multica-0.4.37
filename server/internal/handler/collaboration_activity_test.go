package handler

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestCollaborationActivityHTTPPaginationAndPrivateBoundary(t *testing.T) {
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	external := dbfx.User(t, "External history viewer", "activity-"+uuid.NewString()+"@example.test")
	dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": external}, "issue_id=$1 AND user_id=$2", issue, external)
	for _, seq := range []int64{9007199254740993, 9007199254740994} {
		dbfx.Insert(t, "collaboration_event", testutil.Cols{"issue_id": issue, "event_seq": seq, "actor_user_id": external, "kind": "created", "payload": json.RawMessage(`{"secret":"PRIVATE_ACTIVITY_BODY"}`)})
	}
	request := collaborationContentRequest(http.MethodGet, issue, "", external, "", nil)
	request.URL.RawQuery = "limit=1"
	var first struct {
		Items []service.CollaborationActivityEntry `json:"items"`
		Next  *string                              `json:"next_cursor"`
	}
	response := testutil.Call(t, testHandler.ListCollaborationActivity, request).Want(http.StatusOK)
	response.JSON(&first)
	if len(first.Items) != 1 || first.Next == nil || first.Items[0].EventSeq != 9007199254740994 || first.Items[0].Actor == nil || !first.Items[0].Actor.External {
		t.Fatalf("page lost identity or actor projection: %+v", first)
	}
	if response.Header().Get("Cache-Control") != "no-store" || request.Header.Get("X-Workspace-ID") != "" || strings.Contains(response.Body.String(), "PRIVATE_ACTIVITY_BODY") || strings.Contains(response.Body.String(), "@example.test") {
		t.Fatal("activity leaked stored data or ambient workspace")
	}
	request = collaborationContentRequest(http.MethodGet, issue, "", external, "", nil)
	request.URL.RawQuery = "limit=1&cursor=" + *first.Next
	var next struct {
		Items []service.CollaborationActivityEntry `json:"items"`
	}
	testutil.Call(t, testHandler.ListCollaborationActivity, request).Want(http.StatusOK).JSON(&next)
	if len(next.Items) != 1 || next.Items[0].EventSeq != 9007199254740993 {
		t.Fatalf("page duplicated or rounded events: %+v", next)
	}
	other := collaborationBoundaryIssue(t, testWorkspaceID)
	request = collaborationContentRequest(http.MethodGet, other, "", testUserID, "", nil)
	request.URL.RawQuery = "cursor=" + *first.Next
	testutil.Call(t, testHandler.ListCollaborationActivity, request).Want(http.StatusBadRequest)
	dbfx.Exec(t, `UPDATE issue_participant SET revoked_at=now() WHERE issue_id=$1 AND user_id=$2`, issue, external)
	testutil.Call(t, testHandler.ListCollaborationActivity, collaborationContentRequest(http.MethodGet, issue, "", external, "", nil)).Want(http.StatusNotFound)
}

func TestCollaborationActivityHTTPRejectsInvalidCursors(t *testing.T) {
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	for _, cursor := range []string{
		`{"issue_id":"` + issue + `","before_seq":"01"}`,
		`{"issue_id":"` + issue + `","before_seq":3}`,
		`{"issue_id":"` + issue + `","before_seq":"3","before_seq":"4"}`,
		`{"issue_id":"` + issue + `","before_seq":"3","secret":"hidden"}`,
	} {
		request := collaborationContentRequest(http.MethodGet, issue, "", testUserID, "", nil)
		request.URL.RawQuery = "cursor=" + base64.RawURLEncoding.EncodeToString([]byte(cursor))
		testutil.Call(t, testHandler.ListCollaborationActivity, request).Want(http.StatusBadRequest)
	}
}
