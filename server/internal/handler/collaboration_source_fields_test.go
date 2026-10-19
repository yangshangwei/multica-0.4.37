package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestCollaborationCreationParentSearchAcceptsMultibyteCharacters(t *testing.T) {
	h, fx, actor, _ := collaborationCreationHTTPFixture(t)
	search := strings.Repeat("研", 200)
	parent := fx.Issue(t, search+" parent")
	for _, tc := range []struct {
		name, search string
		status       int
	}{
		{"chinese_at_limit", search, http.StatusOK},
		{"ascii_at_limit", strings.Repeat("a", 200), http.StatusOK},
		{"chinese_over_limit", search + "研", http.StatusBadRequest},
		{"ascii_over_limit", strings.Repeat("a", 201), http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			query := url.Values{"source_workspace_id": {fx.WorkspaceID}, "parent_search": {tc.search}}
			r := newRequest(http.MethodGet, "/api/collaboration/creation-options?"+query.Encode(), nil)
			r.Header.Set("X-User-ID", actor)
			r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: actor, Kind: auth.CredentialJWT}))
			response := testutil.Call(t, h.GetCollaborationCreationOptions, r).Want(tc.status)
			if tc.name == "chinese_at_limit" {
				var options service.CollaborationCreationOptions
				response.JSON(&options)
				if options.Fields == nil || options.Fields.SourceOptions == nil || len(options.Fields.SourceOptions.Parents) != 1 || options.Fields.SourceOptions.Parents[0].ID != parent {
					t.Fatalf("multibyte search lost its source match: %+v", options.Fields)
				}
			}
		})
	}
}

func TestCollaborationSourceFieldsHTTPPreservesNullAndSeparatesStatus(t *testing.T) {
	issue := collaborationEditFixture(t)
	project := dbfx.Project(t, "Field project")
	body := collaborationEditBodyValues()
	body["project_id"], body["parent_issue_id"], body["stage"] = project, nil, 3
	response := testutil.Call(t, testHandler.UpdateCollaborationIssue, collaborationEditRequest(issue, testUserID, uuid.NewString(), body)).Want(http.StatusOK)
	if response.Map()["code"] != "issue_edited" {
		t.Fatal(response.Body.String())
	}
	var detail CollaborationIssueResponse
	testutil.Call(t, testHandler.GetCollaborationIssue, collaborationFieldsReadRequest(issue, testUserID, "")).Want(http.StatusOK).JSON(&detail)
	if detail.SourceStructure == nil || detail.SourceStructure.Project == nil || detail.SourceStructure.Project.ID != project || detail.SourceStructure.Stage == nil || *detail.SourceStructure.Stage != 3 {
		t.Fatalf("source references not projected: %+v", detail.SourceStructure)
	}
	body["expected_collaboration_revision"], body["stage"], body["project_id"] = "2", nil, nil
	testutil.Call(t, testHandler.UpdateCollaborationIssue, collaborationEditRequest(issue, testUserID, uuid.NewString(), body)).Want(http.StatusOK)
	if dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND stage IS NULL AND project_id IS NULL`, issue) != 1 {
		t.Fatal("explicit null did not clear source fields")
	}
	body["expected_collaboration_revision"], body["status"] = "3", "todo"
	testutil.Call(t, testHandler.UpdateCollaborationIssue, collaborationEditRequest(issue, testUserID, uuid.NewString(), body)).Want(http.StatusBadRequest)
}

func TestCollaborationCreationFieldsHTTPAndInitialIteration(t *testing.T) {
	h, fx, actor, _ := collaborationCreationHTTPFixture(t)
	period := fx.Insert(t, "iteration", testutil.Cols{"workspace_id": fx.WorkspaceID, "name": "Initial period", "timezone": "UTC", "start_date": "2026-10-01", "end_date": "2026-10-31", "created_by": actor})
	fx.Cleanup(t, `DELETE FROM iteration_event WHERE iteration_id=$1`, period)
	fx.Cleanup(t, `DELETE FROM iteration_participation WHERE iteration_id=$1`, period)
	property := fx.Insert(t, "issue_property", testutil.Cols{"workspace_id": fx.WorkspaceID, "name": "Initial value", "type": "checkbox"})
	body := map[string]any{"source_workspace_id": fx.WorkspaceID, "title": "Created with initial fields", "status": "backlog", "priority": "high", "start_date": "2026-10-19", "due_date": "2026-10-28",
		"properties": map[string]any{property: false}, "current_iteration_id": period, "expected_iteration_revision": "1", "stage": 2}
	var detail CollaborationIssueResponse
	key := uuid.NewString()
	testutil.Call(t, h.CreateCollaborationIssue, collaborationCreationHTTPRequest(http.MethodPost, fx.WorkspaceID, key, actor, body, false)).Want(http.StatusCreated).JSON(&detail)
	if detail.Status != "backlog" || detail.Priority != "high" || detail.StartDate == nil || *detail.StartDate != "2026-10-19" || detail.SourceStructure == nil || detail.SourceStructure.Iteration == nil || detail.SourceStructure.Iteration.ID != period || string(detail.Properties[property]) != "false" {
		t.Fatalf("initial fields not round-tripped: %+v", detail)
	}
	body["expected_iteration_revision"] = 1
	testutil.Call(t, h.CreateCollaborationIssue, collaborationCreationHTTPRequest(http.MethodPost, fx.WorkspaceID, uuid.NewString(), actor, body, false)).Want(http.StatusBadRequest)
	if fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, fx.WorkspaceID) != 1 {
		t.Fatal("malformed initial revision created a second issue")
	}
}
