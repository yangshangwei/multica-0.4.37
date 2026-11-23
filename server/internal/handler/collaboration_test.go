package handler

import (
	"encoding/base64"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestCollaborationAccountReadIgnoresAmbientWorkspaceAndKeepsPrivateIdentity(t *testing.T) {
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	suffix := uuid.NewString()
	external := dbfx.User(t, "External collaborator", "private-email-"+suffix+"@example.test")
	dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": external}, "issue_id=$1 AND user_id=$2", issue, external)
	dbfx.Exec(t, `UPDATE issue SET assignee_type='member',assignee_id=$2 WHERE id=$1`, issue, external)
	dbfx.Exec(t, `UPDATE issue_collaboration SET revision=9007199254740993 WHERE issue_id=$1`, issue)
	dbfx.Exec(t, `UPDATE collaboration_session SET control_version=9007199254740995 WHERE issue_id=$1`, issue)
	r := withURLParam(newRequest(http.MethodGet, "/api/collaboration/issues/"+issue, nil), "id", issue)
	r.Header.Set("X-Workspace-ID", "not-the-source-workspace")
	r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: external, Kind: auth.CredentialJWT}))
	var out CollaborationIssueResponse
	response := testutil.Call(t, testHandler.GetCollaborationIssue, r).Want(http.StatusOK)
	response.JSON(&out)
	if out.ID != issue || out.SourceWorkspace.ID != testWorkspaceID || out.Handler.UserID != external || !out.Handler.External || out.Handler.Name != "External collaborator" {
		t.Fatalf("wrong account/object projection: %+v", out)
	}
	if out.Revision != "9007199254740993" || out.Session.ControlVersion != "9007199254740995" {
		t.Fatalf("lost bigint precision: %+v", out)
	}
	if !out.Capabilities.Edit || out.Capabilities.Complete || out.Capabilities.EditSourceStructure {
		t.Fatalf("wrong external handler rights: %+v", out.Capabilities)
	}
	// Marshal only the explicit DTO: neither user email nor profile/username
	// fields belong to an external collaborator summary.
	if strings.Contains(response.Body.String(), "private-email") || strings.Contains(response.Body.String(), `"email"`) || strings.Contains(response.Body.String(), `"username"`) {
		t.Fatal("private account fields leaked in collaboration response")
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("private collaboration response can be cached")
	}
	for _, principal := range []auth.AuthenticatedPrincipal{
		{UserID: external, Kind: auth.CredentialTask},
		{UserID: external, Kind: auth.CredentialJWT, Restricted: true},
		{UserID: uuid.NewString(), Kind: auth.CredentialJWT},
	} {
		r := withURLParam(newRequest(http.MethodGet, "/api/collaboration/issues/"+issue, nil), "id", issue)
		r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), principal))
		testutil.Call(t, testHandler.GetCollaborationIssue, r).Want(http.StatusNotFound)
	}
}

func TestCollaborationIssueProjectsCalendarDatesForExternalParticipant(t *testing.T) {
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	viewer := dbfx.User(t, "Calendar participant", "calendar-"+uuid.NewString()+"@example.invalid")
	dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": viewer}, "issue_id=$1 AND user_id=$2", issue, viewer)
	read := func() map[string]any {
		t.Helper()
		r := withURLParam(newRequest(http.MethodGet, "/api/collaboration/issues/"+issue, nil), "id", issue)
		r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: viewer, Kind: auth.CredentialJWT}))
		return testutil.Call(t, testHandler.GetCollaborationIssue, r).Want(http.StatusOK).Map()
	}
	for _, key := range []string{"start_date", "due_date"} {
		if value, present := read()[key]; !present || value != nil {
			t.Fatalf("unset %s must be explicit null, got %v (present %v)", key, value, present)
		}
	}
	dbfx.Exec(t, `UPDATE issue SET start_date='2026-10-14',due_date='2026-10-31' WHERE id=$1`, issue)
	populated := read()
	if populated["start_date"] != "2026-10-14" || populated["due_date"] != "2026-10-31" {
		t.Fatalf("dates must use the ordinary calendar-date wire format: %v", populated)
	}
	dbfx.Exec(t, `UPDATE issue SET due_date=NULL WHERE id=$1`, issue)
	cleared := read()
	if cleared["start_date"] != "2026-10-14" || cleared["due_date"] != nil {
		t.Fatalf("clearing due date changed the independent start date: %v", cleared)
	}
}

func TestCollaborationWorkbenchKeepsResponsibilityParticipationAndPaginationSeparate(t *testing.T) {
	current := collaborationBoundaryIssue(t, testWorkspaceID)
	done := collaborationBoundaryIssue(t, testWorkspaceID)
	dbfx.Exec(t, `UPDATE issue SET status='done' WHERE id=$1`, done)
	transferred := collaborationBoundaryIssue(t, testWorkspaceID)
	external := dbfx.User(t, "Workbench handler", "workbench-"+uuid.NewString()+"@example.test")
	dbfx.Exec(t, `UPDATE issue SET assignee_id=$2 WHERE id=$1`, transferred, external)
	dbfx.Exec(t, `UPDATE issue SET updated_at='2026-10-13T01:00:00Z' WHERE id=ANY($1::uuid[])`, []string{current, done, transferred})
	memberOnly := collaborationBoundaryIssue(t, testWorkspaceID)
	dbfx.Exec(t, `DELETE FROM issue_participant WHERE issue_id=$1 AND user_id=$2`, memberOnly, testUserID)
	dbfx.Issue(t, "Ordinary issue is not a collaboration workbench item")
	call := func(query string) CollaborationWorkbenchResponse {
		t.Helper()
		r := newRequest(http.MethodGet, "/api/me/workbench?"+query, nil)
		r.Header.Set("X-Workspace-ID", "invalid-ambient-workspace")
		r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT}))
		var result CollaborationWorkbenchResponse
		testutil.Call(t, testHandler.GetCollaborationWorkbench, r).Want(http.StatusOK).JSON(&result)
		return result
	}
	active := call("view=current")
	if active.Total != 1 || len(active.Items) != 1 || active.Items[0].ID != current {
		t.Fatalf("current responsibility list: %+v", active)
	}
	page := call("view=participating&limit=1")
	if page.Total != 3 || len(page.Items) != 1 || page.NextCursor == nil {
		t.Fatalf("participation first page: %+v", page)
	}
	seen := map[string]bool{page.Items[0].ID: true}
	for page.NextCursor != nil {
		page = call("view=participating&limit=1&cursor=" + *page.NextCursor)
		if page.Total != 3 {
			t.Fatalf("cursor changed total from visible predicate: %+v", page)
		}
		for _, item := range page.Items {
			if seen[item.ID] {
				t.Fatal("pagination repeated identity")
			}
			seen[item.ID] = true
		}
	}
	if len(seen) != 3 || !seen[current] || !seen[done] || !seen[transferred] {
		t.Fatalf("lost participating history: %+v", seen)
	}
	oldCursor := base64.RawURLEncoding.EncodeToString([]byte(`{"updated_at":"1970-01-01T00:00:00Z","id":"00000000-0000-4000-8000-000000000001"}`))
	empty := call("view=participating&limit=1&cursor=" + oldCursor)
	if len(empty.Items) != 0 || empty.Total != 3 || empty.NextCursor != nil {
		t.Fatalf("empty page lost total or array shape: %+v", empty)
	}
	for _, query := range []string{"view=unknown", "limit=0", "cursor=not-a-cursor"} {
		r := newRequest(http.MethodGet, "/api/me/workbench?"+query, nil)
		r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT}))
		testutil.Call(t, testHandler.GetCollaborationWorkbench, r).Want(http.StatusBadRequest)
	}
}

func TestCollaborationWorkbenchWorksWithoutAnyWorkspaceMembership(t *testing.T) {
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	viewer := dbfx.User(t, "No-workspace participant", "no-space-"+uuid.NewString()+"@example.test")
	dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": viewer}, "issue_id=$1 AND user_id=$2", issue, viewer)
	dbfx.Exec(t, `UPDATE issue SET assignee_id=$2 WHERE id=$1`, issue, viewer)
	request := func() *http.Request {
		r := newRequest(http.MethodGet, "/api/me/workbench", nil)
		return r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: viewer, Kind: auth.CredentialJWT}))
	}
	var page CollaborationWorkbenchResponse
	testutil.Call(t, testHandler.GetCollaborationWorkbench, request()).Want(http.StatusOK).JSON(&page)
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != issue {
		t.Fatalf("zero-space account cannot see assigned collaboration: %+v", page)
	}
	dbfx.Exec(t, `UPDATE issue_participant SET revoked_at=now(),epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, issue, viewer)
	testutil.Call(t, testHandler.GetCollaborationWorkbench, request()).Want(http.StatusOK).JSON(&page)
	if page.Total != 0 || len(page.Items) != 0 {
		t.Fatalf("revoked participant retained workbench data: %+v", page)
	}
	dbfx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, viewer)
	testutil.Call(t, testHandler.GetCollaborationWorkbench, request()).Want(http.StatusForbidden)
}
