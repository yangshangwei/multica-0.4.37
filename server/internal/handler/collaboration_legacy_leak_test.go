package handler

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// Risk signals filter on status, dates and assignee, which are private to a
// collaboration issue (R53). A machine must not see one in the page or total.
func TestProjectHealthRisksKeepCollaborationFromMachines(t *testing.T) {
	project := healthProject(t)
	shared := collaborationBoundaryIssue(t, testWorkspaceID)
	secret := "private-health-" + uuid.NewString()
	dbfx.Exec(t, `UPDATE issue SET project_id=$2,status='blocked',description=$3 WHERE id=$1`, shared, project, secret)
	ordinary := dbfx.Issue(t, "Ordinary blocked control", testutil.Cols{"project_id": project, "status": "blocked"})
	call := func(t *testing.T, kind auth.CredentialKind) (ProjectRiskPage, string) {
		t.Helper()
		r := withURLParam(newRequest(http.MethodGet, "/api/projects/"+project+"/health/issues?signal=blocked", nil), "id", project)
		if kind != "missing_context" {
			r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: kind}))
		}
		var out ProjectRiskPage
		response := testutil.Call(t, testHandler.GetProjectHealthIssues, r).Want(http.StatusOK)
		response.JSON(&out)
		return out, response.Body.String()
	}
	for _, kind := range []auth.CredentialKind{auth.CredentialTask, auth.CredentialCloud, "future_machine", "missing_context"} {
		t.Run(string(kind), func(t *testing.T) {
			out, body := call(t, kind)
			if out.Total != 1 || len(out.Items) != 1 || out.Items[0].ID != ordinary || strings.Contains(body, shared) || strings.Contains(body, secret) {
				t.Fatalf("machine risk page exposed collaboration issue: %s", body)
			}
		})
	}
	out, body := call(t, auth.CredentialJWT)
	if out.Total != 2 || !strings.Contains(body, shared) {
		t.Fatalf("human risk page lost a readable collaboration issue: %s", body)
	}
}

// The duplicate guard used to return a matching collaboration issue in the
// 409 body, so any caller could read its description by guessing its title.
func TestCreateIssueDuplicateGuardIgnoresCollaborationIssue(t *testing.T) {
	shared := collaborationBoundaryIssue(t, testWorkspaceID)
	secret := "private-duplicate-" + uuid.NewString()
	title := "Duplicate probe " + uuid.NewString()
	dbfx.Exec(t, `UPDATE issue SET title=$2,description=$3 WHERE id=$1`, shared, title, secret)
	// Fixture rows take MAX(number)+1; the real create path uses the counter.
	dbfx.Exec(t, `UPDATE workspace SET issue_counter=GREATEST(issue_counter,COALESCE((SELECT max(number) FROM issue WHERE workspace_id=$1),0)) WHERE id=$1`, testWorkspaceID)
	create := func(kind auth.CredentialKind, want int) *testutil.Response {
		t.Helper()
		r := newRequest(http.MethodPost, "/api/issues?workspace_id="+testWorkspaceID, map[string]any{"title": title})
		r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: kind}))
		return testutil.Call(t, testHandler.CreateIssue, r).Want(want)
	}
	// A human is no longer blocked by a same-titled collaboration issue.
	var created IssueResponse
	response := create(auth.CredentialJWT, http.StatusCreated)
	response.JSON(&created)
	dbfx.Cleanup(t, `DELETE FROM issue WHERE id=$1`, created.ID)
	if created.ID == shared || strings.Contains(response.Body.String(), secret) {
		t.Fatalf("create reused or exposed the collaboration issue: %s", response.Body.String())
	}
	// The guard still works for ordinary issues. It picks the oldest match, so
	// naming the newer ordinary issue proves the shared one was never a candidate.
	var conflict struct {
		Issue IssueResponse `json:"issue"`
	}
	response = create(auth.CredentialTask, http.StatusConflict)
	response.JSON(&conflict)
	if conflict.Issue.ID != created.ID || strings.Contains(response.Body.String(), shared) || strings.Contains(response.Body.String(), secret) {
		t.Fatalf("ordinary duplicate guard changed: %s", response.Body.String())
	}
}

// Generic uploads must not attach to a collaboration issue or comment: that
// bypasses private storage, bumps revisions other writers hold and broadcasts
// the comment body to the source workspace room (R53, R43, R44).
func TestLegacyUploadCannotAttachToCollaboration(t *testing.T) {
	storage := &mockStorage{}
	original := testHandler.Storage
	testHandler.Storage = storage
	t.Cleanup(func() { testHandler.Storage = original })
	shared := collaborationBoundaryIssue(t, testWorkspaceID)
	comment := dbfx.Comment(t, shared, "Another author's comment")
	ordinary := dbfx.Issue(t, "Ordinary upload control")
	upload := func(t *testing.T, kind auth.CredentialKind, field, id string, want int) *testutil.Response {
		t.Helper()
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("file", "probe.txt")
		if err == nil {
			_, err = part.Write([]byte("probe"))
		}
		if err == nil {
			err = writer.WriteField(field, id)
		}
		if err == nil {
			err = writer.Close()
		}
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(http.MethodPost, "/api/upload-file", &body)
		r.Header.Set("Content-Type", writer.FormDataContentType())
		r.Header.Set("X-User-ID", testUserID)
		r.Header.Set("X-Workspace-ID", testWorkspaceID)
		r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: kind}))
		return testutil.Call(t, testHandler.UploadFile, r).Want(want)
	}
	var issueRevision, commentRevision int64
	dbfx.QueryRow(t, `SELECT revision FROM issue WHERE id=$1`, shared).Scan(&issueRevision)
	dbfx.QueryRow(t, `SELECT revision FROM comment WHERE id=$1`, comment).Scan(&commentRevision)
	for _, kind := range []auth.CredentialKind{auth.CredentialTask, auth.CredentialJWT} {
		for _, target := range []struct{ field, id string }{{"issue_id", shared}, {"comment_id", comment}} {
			if code := upload(t, kind, target.field, target.id, http.StatusConflict).Map()["code"]; code != "unsupported_collaboration_action" {
				t.Fatalf("%s upload via %s: code %v", kind, target.field, code)
			}
		}
	}
	var issueAfter, commentAfter int64
	dbfx.QueryRow(t, `SELECT revision FROM issue WHERE id=$1`, shared).Scan(&issueAfter)
	dbfx.QueryRow(t, `SELECT revision FROM comment WHERE id=$1`, comment).Scan(&commentAfter)
	if issueAfter != issueRevision || commentAfter != commentRevision {
		t.Fatalf("refused upload bumped revisions: issue %d->%d comment %d->%d", issueRevision, issueAfter, commentRevision, commentAfter)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM attachment WHERE issue_id=$1 OR comment_id=$2`, shared, comment); n != 0 || len(storage.files) != 0 {
		t.Fatalf("refused upload left %d attachment rows and %d stored objects", n, len(storage.files))
	}
	dbfx.Cleanup(t, `DELETE FROM attachment WHERE issue_id=$1`, ordinary)
	upload(t, auth.CredentialJWT, "issue_id", ordinary, http.StatusOK)
}
