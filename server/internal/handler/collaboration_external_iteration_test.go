package handler

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestCollaborationExternalHandlerIterationStatisticsPreviewAndHistory(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	h := lifecycleHTTPHandler(t)
	first := lifecycleHTTPCreate(t, h, "External handler current period")
	next := lifecycleHTTPCreate(t, h, "External handler next period")
	issue := collaborationEditFixture(t)
	email := "PRIVATE_ITERATION_HANDLER_" + uuid.NewString() + "@example.test"
	username := "private_iteration_handler_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	external := dbfx.User(t, "External iteration handler", email)
	dbfx.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": external, "username": username, "password_hash": "fixture-never-used-for-login", "must_change_password": false}, "user_id=$1", external)
	dbfx.InsertNoID(t, "collaboration_account_fence", testutil.Cols{"user_id": external}, "user_id=$1", external)
	dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": external}, "issue_id=$1 AND user_id=$2", issue, external)
	dbfx.Exec(t, `UPDATE issue SET assignee_type='member',assignee_id=$2 WHERE id=$1`, issue, external)
	dbfx.Cleanup(t, `DELETE FROM collaboration_notification WHERE issue_id=$1`, issue)
	project := dbfx.Project(t, "Minimal source project")
	parent := dbfx.Issue(t, "Minimal source parent", testutil.Cols{"description": "PRIVATE_PARENT_BODY"})
	body := collaborationEditBodyValues()
	body["project_id"], body["parent_issue_id"] = project, parent
	testutil.Call(t, h.UpdateCollaborationIssue, collaborationEditRequest(issue, testUserID, uuid.NewString(), body)).Want(http.StatusOK)
	row, err := h.Queries.GetIssue(t.Context(), parseUUID(issue))
	if err != nil {
		t.Fatal(err)
	}
	join := collaborationEditBodyValues()
	join["expected_collaboration_revision"], join["expected_revision"] = "2", strconv.FormatInt(row.Revision, 10)
	join["current_iteration_id"], join["expected_iteration_revision"] = first, "1"
	joinRequest := collaborationEditRequest(issue, testUserID, uuid.NewString(), join)
	joinRequest.Method = http.MethodPut
	testutil.Call(t, h.UpdateCollaborationIteration, joinRequest).Want(http.StatusOK)
	assertRedacted := func(response *testutil.Response) {
		t.Helper()
		for _, private := range []string{email, username, `"email"`, `"username"`, "PRIVATE_PARENT_BODY"} {
			if strings.Contains(response.Text(), private) {
				t.Fatalf("iteration/source projection leaked %q", private)
			}
		}
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("private iteration/source projection may be cached")
		}
	}
	var externalDetail CollaborationIssueResponse
	response := testutil.Call(t, h.GetCollaborationIssue, collaborationFieldsReadRequest(issue, external, "")).Want(http.StatusOK)
	response.JSON(&externalDetail)
	assertRedacted(response)
	if externalDetail.SourceStructure == nil || externalDetail.SourceStructure.Iteration == nil || externalDetail.SourceStructure.Iteration.ID != first ||
		externalDetail.SourceStructure.Project == nil || externalDetail.SourceStructure.Project.ID != project || externalDetail.SourceStructure.ParentIssue == nil || externalDetail.SourceStructure.ParentIssue.ID != parent {
		t.Fatal("external handler lost the minimal source project/parent/iteration references")
	}
	var options service.CollaborationFieldOptions
	response = testutil.Call(t, h.GetCollaborationFieldOptions, collaborationFieldsReadRequest(issue, external, "")).Want(http.StatusOK)
	response.JSON(&options)
	assertRedacted(response)
	if options.SourceOptions != nil || dbfx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, external) != 0 {
		t.Fatal("external handler received source catalogs or a fabricated membership")
	}
	periodRow, err := h.Queries.GetIteration(t.Context(), db.GetIterationParams{WorkspaceID: parseUUID(testWorkspaceID), ID: parseUUID(first)})
	if err != nil {
		t.Fatal(err)
	}
	start := iteration.Draft{Operation: "start", IterationID: &first, ExpectedIterationRevision: &periodRow.Revision,
		ExpectedScopeRevision: &periodRow.ScopeRevision, ExpectedSettingsRevision: 2, Moves: []iteration.Move{},
		Start: &iteration.StartDraft{TargetID: first, Mode: "scheduled", TerminalChoices: []iteration.TerminalChoice{}}}
	var preview iteration.Preview
	response = testutil.Call(t, h.PreviewIterationOperation, lifecycleHTTPRequest(http.MethodPost, "iteration-previews", "", start)).Want(http.StatusOK)
	response.JSON(&preview)
	assertRedacted(response)
	if !preview.Complete || len(preview.InvalidItems) != 0 || len(preview.Issues) != 1 || preview.Issues[0].IssueID != issue ||
		preview.Issues[0].Assignee.ID == nil || *preview.Issues[0].Assignee.ID != external || preview.Issues[0].Assignee.Type == nil || *preview.Issues[0].Assignee.Type != "member" ||
		preview.Statistics[first].Current != 1 {
		t.Fatalf("external-assignee start preview/statistics unavailable: %+v", preview)
	}
	// No source membership means no source-directory assignee name. Its nullable
	// display reference must not make the whole planning/history flow unusable.
	if preview.Issues[0].Assignee.Name != nil || preview.Issues[0].Assignee.Available {
		t.Fatal("ordinary iteration preview widened the source member directory")
	}
	for _, handler := range []http.HandlerFunc{h.GetIteration, h.ListIterationIssues, h.ListIterationEvents} {
		r := lifecycleHTTPRequest(http.MethodGet, "iterations/"+first, first, nil)
		r.Header.Set("X-User-ID", external)
		denied := testutil.Call(t, handler, r).Want(http.StatusForbidden)
		assertRedacted(denied)
	}
	externalPreview := lifecycleHTTPRequest(http.MethodPost, "iteration-previews", "", start)
	externalPreview.Header.Set("X-User-ID", external)
	assertRedacted(testutil.Call(t, h.PreviewIterationOperation, externalPreview).Want(http.StatusForbidden))
	var started iteration.WriteResult
	response = testutil.Call(t, h.ApplyIterationOperation, lifecycleHTTPRequest(http.MethodPost, "iteration-operations", "",
		service.ApplyIterationInput{RequestID: uuid.NewString(), Draft: preview.Draft, PreviewHash: preview.PreviewHash})).Want(http.StatusOK)
	response.JSON(&started)
	assertRedacted(response)
	var detail struct {
		Statistics iteration.Statistics `json:"statistics"`
		Snapshot   *iteration.Snapshot  `json:"snapshot"`
	}
	response = testutil.Call(t, h.GetIteration, lifecycleHTTPRequest(http.MethodGet, "iterations/"+first, first, nil)).Want(http.StatusOK)
	response.JSON(&detail)
	assertRedacted(response)
	if detail.Statistics.Original != 1 || detail.Statistics.Current != 1 || detail.Snapshot != nil {
		t.Fatalf("external handler was dropped from active statistics: %+v", detail)
	}
	var items struct {
		Items []iteration.HistoricalIssue `json:"items"`
	}
	response = testutil.Call(t, h.ListIterationIssues, lifecycleHTTPRequest(http.MethodGet, "iterations/"+first+"/issues", first, nil)).Want(http.StatusOK)
	response.JSON(&items)
	assertRedacted(response)
	if len(items.Items) != 1 || items.Items[0].IssueID != issue || items.Items[0].AssigneeID == nil || *items.Items[0].AssigneeID != external || items.Items[0].AssigneeName != nil {
		t.Fatalf("external handler broke the current historical issue projection: %+v", items.Items)
	}
	row, err = h.Queries.GetIssue(t.Context(), parseUUID(issue))
	if err != nil {
		t.Fatal(err)
	}
	periodRow, err = h.Queries.GetIteration(t.Context(), db.GetIterationParams{WorkspaceID: parseUUID(testWorkspaceID), ID: parseUUID(first)})
	if err != nil {
		t.Fatal(err)
	}
	reason := "Rollover external collaboration according to source planning"
	end := iteration.Draft{Operation: "end", IterationID: &first, ExpectedIterationRevision: &periodRow.Revision,
		ExpectedScopeRevision: &periodRow.ScopeRevision, ExpectedSettingsRevision: 2, Reason: &reason,
		Moves: []iteration.Move{{IssueID: issue, ExpectedIssueRevision: row.Revision, ExpectedSourceID: &first, TargetID: &next}}}
	var endedPreview iteration.Preview
	response = testutil.Call(t, h.PreviewIterationOperation, lifecycleHTTPRequest(http.MethodPost, "iteration-previews", "", end)).Want(http.StatusOK)
	response.JSON(&endedPreview)
	assertRedacted(response)
	if !endedPreview.Complete || len(endedPreview.InvalidItems) != 0 || len(endedPreview.Issues) != 1 || endedPreview.Issues[0].IssueID != issue {
		t.Fatalf("external handler blocked server rollover preview: %+v", endedPreview)
	}
	response = testutil.Call(t, h.ApplyIterationOperation, lifecycleHTTPRequest(http.MethodPost, "iteration-operations", "",
		service.ApplyIterationInput{RequestID: uuid.NewString(), Draft: endedPreview.Draft, PreviewHash: endedPreview.PreviewHash})).Want(http.StatusOK)
	assertRedacted(response)
	response = testutil.Call(t, h.GetIteration, lifecycleHTTPRequest(http.MethodGet, "iterations/"+first, first, nil)).Want(http.StatusOK)
	response.JSON(&detail)
	assertRedacted(response)
	if detail.Snapshot == nil || detail.Statistics.Original != 1 || detail.Statistics.Current != 1 || len(detail.Snapshot.Destinations) != 1 ||
		detail.Snapshot.Destinations[0].IssueID != issue || detail.Snapshot.Destinations[0].TargetIterationID == nil || *detail.Snapshot.Destinations[0].TargetIterationID != next ||
		detail.Snapshot.Destinations[0].RolloverCountAfter != 1 {
		t.Fatalf("external collaboration rollover did not freeze complete statistics/history: %+v", detail)
	}
	response = testutil.Call(t, h.ListIterationEvents, lifecycleHTTPRequest(http.MethodGet, "iterations/"+first+"/events", first, nil)).Want(http.StatusOK)
	var events struct {
		Items []iteration.Event `json:"items"`
	}
	response.JSON(&events)
	assertRedacted(response)
	if len(events.Items) == 0 || dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND current_iteration_id=$2 AND iteration_rollover_count=1 AND assignee_id=$3`, issue, next, external) != 1 ||
		dbfx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id IN (SELECT id FROM collaboration_session WHERE issue_id=$1)`, issue) != 0 {
		t.Fatal("external handler history is empty, responsibility changed, or source planning triggered AI")
	}
}
