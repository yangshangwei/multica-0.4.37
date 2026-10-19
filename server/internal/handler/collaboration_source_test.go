package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestSourceCollaborationProjectionAndExternalFilter(t *testing.T) {
	shared := collaborationBoundaryIssue(t, testWorkspaceID)
	ordinary := dbfx.Issue(t, "Ordinary source companion")
	external := dbfx.User(t, "Outside collaborator", "source-private-"+uuid.NewString()+"@example.test")
	dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": shared, "user_id": external}, "issue_id=$1 AND user_id=$2", shared, external)
	dbfx.Exec(t, `UPDATE issue SET assignee_id=$2 WHERE id=$1`, shared, external)
	request := func(method, path string, body any) *http.Request {
		r := newRequest(method, path, body)
		return r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT}))
	}
	assertProjection := func(row map[string]any) {
		t.Helper()
		projection, ok := row["collaboration"].(map[string]any)
		if !ok || projection["has_external_collaborators"] != true {
			t.Fatalf("source row lost collaboration identity: %v", row)
		}
		handler, ok := projection["handler"].(map[string]any)
		if !ok || handler["user_id"] != external || handler["name"] != "Outside collaborator" || handler["external"] != true {
			t.Fatalf("external handler missing minimal summary: %v", projection)
		}
		encoded, err := json.Marshal(projection)
		if err != nil || strings.Contains(string(encoded), "source-private-") || strings.Contains(string(encoded), `"email"`) || strings.Contains(string(encoded), `"username"`) {
			t.Fatalf("source projection leaked private account fields: %s (%v)", encoded, err)
		}
	}
	for _, open := range []string{"", "&open_only=true"} {
		var page struct {
			Issues []map[string]any `json:"issues"`
			Total  int              `json:"total"`
		}
		testutil.Call(t, testHandler.ListIssues, request(http.MethodGet, "/api/issues?external_collaborators=true"+open, nil)).Want(http.StatusOK).JSON(&page)
		if page.Total != 1 || len(page.Issues) != 1 || page.Issues[0]["id"] != shared {
			t.Fatalf("external filter/page count disagree: %+v", page)
		}
		assertProjection(page.Issues[0])
	}
	detail := testutil.Call(t, testHandler.GetIssue, withURLParam(request(http.MethodGet, "/api/issues/"+shared, nil), "id", shared)).Want(http.StatusOK).Map()
	assertProjection(detail)
	var queried struct {
		Issues []map[string]any `json:"issues"`
		Total  int              `json:"total"`
	}
	testutil.Call(t, testHandler.QueryIssues, request(http.MethodPost, "/api/issues/query", map[string]string{"external_collaborators": "true"})).Want(http.StatusOK).JSON(&queried)
	if queried.Total != 1 || len(queried.Issues) != 1 {
		t.Fatalf("POST filter/count mismatch: %+v", queried)
	}
	assertProjection(queried.Issues[0])
	testutil.Call(t, testHandler.SearchIssues, request(http.MethodGet, "/api/issues/search?q=Protected", nil)).Want(http.StatusOK).JSON(&queried)
	if queried.Total != 1 || len(queried.Issues) != 1 {
		t.Fatalf("search projection missing: %+v", queried)
	}
	assertProjection(queried.Issues[0])
	var grouped struct {
		Groups []struct {
			Issues []map[string]any `json:"issues"`
		} `json:"groups"`
	}
	testutil.Call(t, testHandler.ListGroupedIssues, request(http.MethodGet, "/api/issues/grouped?group_by=assignee&external_collaborators=true", nil)).Want(http.StatusOK).JSON(&grouped)
	if len(grouped.Groups) != 1 || len(grouped.Groups[0].Issues) != 1 {
		t.Fatalf("grouped source filter mismatch: %+v", grouped)
	}
	assertProjection(grouped.Groups[0].Issues[0])
	reordered := testutil.Call(t, testHandler.UpdateIssue, withURLParam(request(http.MethodPut, "/api/issues/"+shared, map[string]any{"position": -2}), "id", shared)).Want(http.StatusOK).Map()
	assertProjection(reordered)
	var collaborationRevision int64
	dbfx.QueryRow(t, `SELECT revision FROM issue_collaboration WHERE issue_id=$1`, shared).Scan(&collaborationRevision)
	if collaborationRevision != 1 || dbfx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, shared) != 0 {
		t.Fatal("source layout reorder advanced workflow history or version")
	}
	regular := testutil.Call(t, testHandler.GetIssue, withURLParam(request(http.MethodGet, "/api/issues/"+ordinary, nil), "id", ordinary)).Want(http.StatusOK).Map()
	if regular["collaboration"] != nil {
		t.Fatal("ordinary task was marked as collaboration")
	}

	// Being an external participant matters even after handling returns to a source member.
	dbfx.Exec(t, `UPDATE issue SET assignee_id=$2 WHERE id=$1`, shared, testUserID)
	var externalPage struct {
		Issues []map[string]any `json:"issues"`
		Total  int              `json:"total"`
	}
	testutil.Call(t, testHandler.ListIssues, request(http.MethodGet, "/api/issues?external_collaborators=true", nil)).Want(http.StatusOK).JSON(&externalPage)
	if externalPage.Total != 1 {
		t.Fatal("filter lost an external participant after a transfer")
	}
	dbfx.Exec(t, `UPDATE issue_participant SET revoked_at=now() WHERE issue_id=$1 AND user_id=$2`, shared, external)
	testutil.Call(t, testHandler.ListIssues, request(http.MethodGet, "/api/issues?external_collaborators=true", nil)).Want(http.StatusOK).JSON(&externalPage)
	if externalPage.Total != 0 {
		t.Fatal("revoked external participation still matched the filter")
	}
}

func TestSourceCollaborationTableAndChildMachineBoundary(t *testing.T) {
	shared := collaborationBoundaryIssue(t, testWorkspaceID)
	parent := dbfx.Issue(t, "Source parent")
	ordinary := dbfx.Issue(t, "Ordinary visible control", testutil.Cols{"parent_issue_id": parent, "description": "ordinary body"})
	external := dbfx.User(t, "Table external handler", "table-private-"+uuid.NewString()+"@example.test")
	dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": shared, "user_id": external}, "issue_id=$1 AND user_id=$2", shared, external)
	dbfx.Exec(t, `UPDATE issue SET parent_issue_id=$2,assignee_id=$3,description='hidden collaboration body' WHERE id=$1`, shared, parent, external)
	request := func(method, path string, body any, kind auth.CredentialKind) *http.Request {
		r := newRequest(method, path, body)
		return r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: kind}))
	}
	spec := issueTableQuerySpec{Scope: issueTableScope{Kind: "workspace"}, Filters: issueTableFiltersRequest{ExternalCollaborators: true}, Sort: issueTableSortRequest{Field: "position", Direction: "asc"}}
	rowsBody := issueTableRowsRequest{Query: spec, Group: issueTableGroupSpec{Kind: "none"}}
	var rows issueTableRowsResponse
	testutil.Call(t, testHandler.ListIssueTableRows, request(http.MethodPost, "/api/issues/table/rows", rowsBody, auth.CredentialJWT)).Want(http.StatusOK).JSON(&rows)
	if rows.Total != 1 || len(rows.Rows) != 1 || rows.Rows[0].Issue.ID != shared || rows.Rows[0].Issue.Collaboration == nil {
		t.Fatalf("table source filter/projection mismatch: %+v", rows)
	}
	var groups issueTableGroupsResponse
	testutil.Call(t, testHandler.ListIssueTableGroups, request(http.MethodPost, "/api/issues/table/groups", issueTableGroupsRequest{Query: spec, Group: issueTableGroupSpec{Kind: "assignee"}}, auth.CredentialJWT)).Want(http.StatusOK).JSON(&groups)
	if groups.Total != 1 || len(groups.Groups) != 1 || groups.Groups[0].Value.Account == nil || groups.Groups[0].Value.Account.Name != "Table external handler" || !groups.Groups[0].Value.Account.External {
		t.Fatalf("group header lost safe external account: %+v", groups)
	}

	// Structured table dimensions may expose private membership/counts, so
	// machine tables exclude collaboration rows. Generic title search still
	// returns its existing identity-only reference (covered in boundary_test).
	for _, search := range []string{"", "hidden collaboration body", "Protected collaboration"} {
		rowsBody.Query.Filters.ExternalCollaborators = false
		rowsBody.Query.Search = search
		var result issueTableRowsResponse
		testutil.Call(t, testHandler.ListIssueTableRows, request(http.MethodPost, "/api/issues/table/rows", rowsBody, auth.CredentialTask)).Want(http.StatusOK).JSON(&result)
		if search != "" && (result.Total != 0 || len(result.Rows) != 0) {
			t.Fatalf("machine table matched shared text/count: %+v", result)
		}
		if search == "" && result.Total != 2 {
			t.Fatalf("machine table lost ordinary records: %+v", result)
		}
		for _, row := range result.Rows {
			if row.Issue.ID == shared || row.Issue.Collaboration != nil {
				t.Fatal("machine table exposed shared state")
			}
			if row.Issue.ID == ordinary && (row.Issue.Description == nil || *row.Issue.Description != "ordinary body") {
				t.Fatal("ordinary machine projection changed")
			}
		}
	}
	for _, kind := range []auth.CredentialKind{auth.CredentialJWT, auth.CredentialTask} {
		for _, batch := range []bool{false, true} {
			r := request(http.MethodGet, "/api/issues/"+parent+"/children", nil, kind)
			r = withURLParam(r, "id", parent)
			handler := testHandler.ListChildIssues
			if batch {
				r = request(http.MethodGet, "/api/issues/children?parent_ids="+parent, nil, kind)
				handler = testHandler.ListChildrenByParents
			}
			var result struct {
				Issues []map[string]any `json:"issues"`
			}
			testutil.Call(t, handler, r).Want(http.StatusOK).JSON(&result)
			if len(result.Issues) != 2 {
				t.Fatalf("child identities changed: %+v", result)
			}
			for _, row := range result.Issues {
				if row["id"] == shared {
					if kind == auth.CredentialJWT && row["collaboration"] == nil {
						t.Fatal("source child lost shared contract")
					}
					if kind == auth.CredentialTask && (row["collaboration_reference"] != true || row["description"] != nil || row["collaboration"] != nil || row["assignee_id"] != nil) {
						t.Fatalf("machine child leaked private content/account: %v", row)
					}
				}
			}
		}
	}
}
