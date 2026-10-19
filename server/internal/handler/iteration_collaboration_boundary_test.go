package handler

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestIterationHTTPExplicitCollaborationMoveRequiresDedicatedWorkflow(t *testing.T) {
	h := lifecycleHTTPHandler(t)
	period := lifecycleHTTPCreate(t, h, "Mixed source period")
	shared := collaborationEditFixture(t)
	ordinary := dbfx.Issue(t, "Ordinary mixed source issue")
	member := dbfx.User(t, "Source member", uuid.NewString()+"@test.invalid")
	dbfx.Member(t, testWorkspaceID, member, "member")
	join := collaborationEditBodyValues()
	join["current_iteration_id"], join["expected_iteration_revision"], join["expected_revision"] = period, "1", "1"
	request := collaborationEditRequest(shared, testUserID, uuid.NewString(), join)
	request.Method = http.MethodPut
	testutil.Call(t, h.UpdateCollaborationIteration, request).Want(http.StatusOK)
	lifecycleHTTPApply(t, h, iteration.Draft{Operation: "move", ExpectedSettingsRevision: 2, Moves: []iteration.Move{{IssueID: ordinary, ExpectedIssueRevision: 1, TargetID: &period}}})
	reason := "Remove both selected issues"
	draft := iteration.Draft{Operation: "move", ExpectedSettingsRevision: 2, Reason: &reason, Moves: []iteration.Move{
		{IssueID: ordinary, ExpectedIssueRevision: 2, ExpectedSourceID: &period}, {IssueID: shared, ExpectedIssueRevision: 2, ExpectedSourceID: &period},
	}}
	for _, actor := range []string{member, testUserID} {
		for _, path := range []string{"iteration-previews", "iteration-operations"} {
			var body any = draft
			handle := h.PreviewIterationOperation
			if path == "iteration-operations" {
				body = service.ApplyIterationInput{RequestID: uuid.NewString(), Draft: draft, PreviewHash: strings.Repeat("0", 64)}
				handle = h.ApplyIterationOperation
			}
			r := lifecycleHTTPRequest(http.MethodPost, path, "", body)
			r.Header.Set("X-User-ID", actor)
			response := testutil.Call(t, handle, r).Want(http.StatusConflict)
			if response.Map()["code"] != "unsupported_collaboration_action" || strings.Contains(response.Body.String(), "Protected collaboration") || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("wrong collaboration boundary response: %s", response.Body.String())
			}
		}
	}
	if dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=ANY($1::uuid[]) AND current_iteration_id=$2 AND revision=2`, []string{ordinary, shared}, period) != 2 {
		t.Fatal("wrong-endpoint mixed move changed an issue")
	}
}
