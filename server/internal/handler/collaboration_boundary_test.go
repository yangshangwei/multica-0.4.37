package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func collaborationBoundaryIssue(t *testing.T, workspaceID string) string {
	t.Helper()
	issueID := dbfx.Issue(t, "Protected collaboration", testutil.Cols{"workspace_id": workspaceID, "assignee_type": "member", "assignee_id": testUserID})
	dbfx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": issueID}, "issue_id=$1", issueID)
	dbfx.Insert(t, "collaboration_session", testutil.Cols{"issue_id": issueID})
	dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issueID, "user_id": testUserID}, "issue_id=$1 AND user_id=$2", issueID, testUserID)
	return issueID
}

func TestLegacyIssueWritesCannotBypassCollaborationLifecycle(t *testing.T) {
	issueID := collaborationBoundaryIssue(t, testWorkspaceID)
	for _, tc := range []struct {
		method, body string
		handler      http.HandlerFunc
	}{
		{http.MethodPut, `{"status":"done"}`, testHandler.UpdateIssue},
		{http.MethodPut, `{"assignee_id":null}`, testHandler.UpdateIssue},
		{http.MethodDelete, ``, testHandler.DeleteIssue},
	} {
		r := withURLParam(newRequest(tc.method, "/api/issues/"+issueID, json.RawMessage(tc.body)), "id", issueID)
		testutil.Call(t, tc.handler, r).Want(http.StatusConflict)
	}
	var status string
	dbfx.QueryRow(t, `SELECT status FROM issue WHERE id=$1`, issueID).Scan(&status)
	if status != "todo" {
		t.Fatalf("legacy mutation changed collaboration status to %s", status)
	}
}

func TestLegacyMixedBatchRefusesBeforeAnyCollaborationSideEffects(t *testing.T) {
	shared := collaborationBoundaryIssue(t, testWorkspaceID)
	ordinary := dbfx.Issue(t, "Ordinary batch companion")
	for _, tc := range []struct {
		path, body string
		handler    http.HandlerFunc
	}{
		{"batch-update", fmt.Sprintf(`{"issue_ids":[%q,%q],"updates":{"status":"done"}}`, ordinary, shared), testHandler.BatchUpdateIssues},
		{"batch-delete", fmt.Sprintf(`{"issue_ids":[%q,%q]}`, ordinary, shared), testHandler.BatchDeleteIssues},
	} {
		testutil.Call(t, tc.handler, newRequest(http.MethodPost, "/api/issues/"+tc.path, json.RawMessage(tc.body))).Want(http.StatusConflict)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=ANY($1::uuid[]) AND status='todo'`, []string{ordinary, shared}); n != 2 {
		t.Fatalf("mixed batch partially changed/deleted issues: %d", n)
	}
}

func TestWorkspaceWithCollaborationHistoryCannotBeDeleted(t *testing.T) {
	ws := dbfx.Workspace(t, "Retained collaboration workspace", "collab-retained-"+uuid.NewString())
	dbfx.Member(t, ws, testUserID, "owner")
	issue := collaborationBoundaryIssue(t, ws)
	dbfx.Exec(t, `UPDATE issue SET status='done' WHERE id=$1`, issue)
	r := withURLParam(newRequest(http.MethodDelete, "/api/workspaces/"+ws, nil), "id", ws)
	testutil.Call(t, testHandler.DeleteWorkspace, r).Want(http.StatusConflict)
	if dbfx.Count(t, `SELECT count(*) FROM workspace WHERE id=$1`, ws) != 1 || dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1`, issue) != 1 {
		t.Fatal("rejected workspace deletion removed retained data")
	}
}

func TestCollaborationPositionUsesSourceAuthorityWithoutChangingCollaborationVersion(t *testing.T) {
	id := collaborationBoundaryIssue(t, testWorkspaceID)
	r := withURLParam(newRequest(http.MethodPut, "/api/issues/"+id, json.RawMessage(`{"position":-42}`)), "id", id)
	r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT}))
	testutil.Call(t, testHandler.UpdateIssue, r).Want(http.StatusOK)
	var position float64
	var revision int64
	dbfx.QueryRow(t, `SELECT i.position,c.revision FROM issue i JOIN issue_collaboration c ON c.issue_id=i.id WHERE i.id=$1`, id).Scan(&position, &revision)
	if position != -42 || revision != 1 {
		t.Fatalf("layout change: position=%v collaboration_revision=%d", position, revision)
	}
	r = withURLParam(newRequest(http.MethodPut, "/api/issues/"+id, json.RawMessage(`{"position":-50}`)), "id", id)
	r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialTask}))
	testutil.Call(t, testHandler.UpdateIssue, r).Want(http.StatusForbidden)
}

func TestLegacyCollaborationCommentAndRerunPathsStayInert(t *testing.T) {
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	comment := dbfx.Comment(t, issue, "Retained comment")
	for _, tc := range []struct {
		method, path, param, id string
		body                    any
		handler                 http.HandlerFunc
	}{
		{http.MethodPost, "/api/issues/" + issue + "/comments", "id", issue, map[string]any{"content": "Do not enqueue this"}, testHandler.CreateComment},
		{http.MethodPost, "/api/issues/" + issue + "/rerun", "id", issue, nil, testHandler.RerunIssue},
		{http.MethodPut, "/api/comments/" + comment, "commentId", comment, map[string]any{"content": "Do not overwrite"}, testHandler.UpdateComment},
		{http.MethodDelete, "/api/comments/" + comment, "commentId", comment, nil, testHandler.DeleteComment},
		{http.MethodPost, "/api/comments/" + comment + "/resolve", "commentId", comment, nil, testHandler.ResolveComment},
	} {
		r := withURLParam(newRequest(tc.method, tc.path, tc.body), tc.param, tc.id)
		testutil.Call(t, tc.handler, r).Want(http.StatusConflict)
	}
	var content string
	dbfx.QueryRow(t, `SELECT content FROM comment WHERE id=$1`, comment).Scan(&content)
	if content != "Retained comment" {
		t.Fatal("legacy route changed retained collaboration comment")
	}
	if dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, issue) != 0 {
		t.Fatal("legacy route enqueued ordinary AI work")
	}
}

func TestLegacyIssueDetailDoesNotExposeCollaborationToMachine(t *testing.T) {
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	r := withURLParam(newRequest(http.MethodGet, "/api/issues/"+issue, nil), "id", issue)
	r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialTask}))
	testutil.Call(t, testHandler.GetIssue, r).Want(http.StatusNotFound)
	r = withURLParam(newRequest(http.MethodGet, "/api/issues/"+issue, nil), "id", issue)
	r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT}))
	testutil.Call(t, testHandler.GetIssue, r).Want(http.StatusOK)
}

func TestCollaborationMachineSearchAndListKeepOnlyReferences(t *testing.T) {
	shared := collaborationBoundaryIssue(t, testWorkspaceID)
	ordinary := dbfx.Issue(t, "Ordinary searchable control")
	secret := "private-description-" + uuid.NewString()
	commentSecret := "private-comment-" + uuid.NewString()
	title := "Reference-" + uuid.NewString()
	dbfx.Exec(t, `UPDATE issue SET title=$2,description=$3,metadata='{"private":"value"}' WHERE id=$1`, shared, title, secret)
	dbfx.Exec(t, `UPDATE issue SET description=$2 WHERE id=$1`, ordinary, secret)
	dbfx.Comment(t, shared, commentSecret)
	type page struct {
		Issues []map[string]json.RawMessage `json:"issues"`
		Total  int64                        `json:"total"`
	}
	call := func(t *testing.T, kind auth.CredentialKind, handler http.HandlerFunc, path string) (page, string) {
		t.Helper()
		r := newRequest(http.MethodGet, path, nil)
		r.Header.Set("X-Actor-Source", "jwt")
		if kind != "missing_context" {
			r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: kind}))
		}
		var out page
		response := testutil.Call(t, handler, r).Want(http.StatusOK)
		response.JSON(&out)
		return out, response.Body.String()
	}
	for _, kind := range []auth.CredentialKind{auth.CredentialTask, auth.CredentialCloud, "", "future_machine", "missing_context"} {
		t.Run(string(kind), func(t *testing.T) {
			for _, query := range []string{secret, commentSecret} {
				out, body := call(t, kind, testHandler.SearchIssues, "/api/issues/search?q="+url.QueryEscape(query))
				want := int64(0)
				if query == secret {
					want = 1
				}
				if out.Total != want || int64(len(out.Issues)) != want || strings.Contains(body, shared) {
					t.Fatalf("private text changed machine search matches/count: %s", body)
				}
			}
			out, body := call(t, kind, testHandler.SearchIssues, "/api/issues/search?q="+url.QueryEscape(title))
			if out.Total != 1 || len(out.Issues) != 1 {
				t.Fatalf("title reference missing: %s", body)
			}
			for _, endpoint := range []struct {
				path    string
				handler http.HandlerFunc
			}{
				{"/api/issues/search?q=" + url.QueryEscape(title), testHandler.SearchIssues},
				{"/api/issues", testHandler.ListIssues},
				{"/api/issues?open_only=true", testHandler.ListIssues},
			} {
				out, body = call(t, kind, endpoint.handler, endpoint.path)
				found := false
				for _, item := range out.Issues {
					if string(item["id"]) != fmt.Sprintf("%q", shared) {
						continue
					}
					found = true
					if string(item["collaboration_reference"]) != "true" || string(item["can_open"]) != "false" {
						t.Fatalf("machine did not receive a closed reference: %s", body)
					}
					for key := range item {
						switch key {
						case "id", "workspace_id", "number", "identifier", "title", "collaboration_reference", "can_open":
						default:
							t.Fatalf("reference exposed private field %q: %s", key, body)
						}
					}
				}
				if !found {
					t.Fatalf("machine list lost title reference: %s", body)
				}
			}
			for _, filter := range []struct {
				path string
				want int64
			}{
				{"/api/issues?metadata=" + url.QueryEscape(`{"private":"value"}`), 0},
				{"/api/issues?open_only=true&metadata=" + url.QueryEscape(`{"private":"value"}`), 0},
				{"/api/issues?assignee_id=" + testUserID, 0},
				// The ordinary todo control remains visible; only the shared
				// task is excluded from private status filtering.
				{"/api/issues?status=todo", 1},
			} {
				out, body = call(t, kind, testHandler.ListIssues, filter.path)
				if strings.Contains(body, shared) || out.Total != filter.want || int64(len(out.Issues)) != filter.want {
					t.Fatalf("private filter changed permitted results at %s: %s", filter.path, body)
				}
				if filter.want == 1 && string(out.Issues[0]["id"]) != fmt.Sprintf("%q", ordinary) {
					t.Fatalf("private filter returned something other than the ordinary control: %s", body)
				}
			}
			_, body = call(t, kind, testHandler.ListGroupedIssues, "/api/issues/grouped?group_by=assignee")
			if strings.Contains(body, shared) || strings.Contains(body, title) {
				t.Fatalf("private grouping revealed collaboration: %s", body)
			}
		})
	}
	for _, kind := range []auth.CredentialKind{auth.CredentialJWT, auth.CredentialPAT} {
		out, body := call(t, kind, testHandler.SearchIssues, "/api/issues/search?q="+url.QueryEscape(secret))
		if out.Total != 2 || !strings.Contains(body, shared) || !strings.Contains(body, secret) {
			t.Fatalf("human content search changed: %s", body)
		}
	}
}

func TestCollaborationMachineReferencesDoNotRevealStatusOrPrivateOrdering(t *testing.T) {
	first := collaborationBoundaryIssue(t, testWorkspaceID)
	second := collaborationBoundaryIssue(t, testWorkspaceID)
	title := "Stable reference " + uuid.NewString()
	dbfx.Exec(t, `UPDATE issue SET title=$2 WHERE id=ANY($1::uuid[])`, []string{first, second}, title)
	for _, endpoint := range []struct {
		path    string
		handler http.HandlerFunc
	}{
		{"/api/issues?sort=updated_at", testHandler.ListIssues},
		{"/api/issues?open_only=true", testHandler.ListIssues},
		{"/api/issues/search?q=" + url.QueryEscape(title), testHandler.SearchIssues},
	} {
		request := func() *http.Request {
			r := newRequest(http.MethodGet, endpoint.path, nil)
			return r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialTask}))
		}
		before := testutil.Call(t, endpoint.handler, request()).Want(http.StatusOK).Body.String()
		dbfx.Exec(t, `UPDATE issue SET status='done',updated_at=now()+interval '1 day',position=-1000 WHERE id=$1`, first)
		after := testutil.Call(t, endpoint.handler, request()).Want(http.StatusOK).Body.String()
		if before != after {
			t.Fatalf("private status/order changed reference window at %s: before=%s after=%s", endpoint.path, before, after)
		}
		dbfx.Exec(t, `UPDATE issue SET status='todo',updated_at=now(),position=0 WHERE id=$1`, first)
	}
}
