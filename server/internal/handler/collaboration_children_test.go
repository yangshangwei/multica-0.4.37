package handler

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestCollaborationOrdinaryChildrenCompleteAuthorizedProjection(t *testing.T) {
	shared := collaborationBoundaryIssue(t, testWorkspaceID)
	external := dbfx.User(t, "Child reference reader", "children-"+uuid.NewString()+"@example.test")
	dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": shared, "user_id": external}, "issue_id=$1 AND user_id=$2", shared, external)
	// Source members who are neither managers nor participants can open children.
	dbfx.Exec(t, `DELETE FROM issue_participant WHERE issue_id=$1 AND user_id=$2`, shared, testUserID)
	read := func(user string) map[string]any {
		t.Helper()
		r := withURLParam(newRequest(http.MethodGet, "/api/collaboration/issues/"+shared, nil), "id", shared)
		r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: user, Kind: auth.CredentialJWT}))
		result := testutil.Call(t, testHandler.GetCollaborationIssue, r).Want(http.StatusOK)
		if strings.Contains(result.Body.String(), "PRIVATE-CHILD-BODY") {
			t.Fatal("ordinary child body crossed the shared reference boundary")
		}
		block, ok := result.Map()["ordinary_children"].(map[string]any)
		if !ok {
			t.Fatal("authorized detail omitted the ordinary child projection")
		}
		return block
	}
	if items, ok := read(external)["items"].([]any); !ok || len(items) != 0 {
		t.Fatal("supported empty children must be an explicit array")
	}
	ids := make([]string, 205)
	for i := range ids {
		ids[i] = dbfx.Issue(t, fmt.Sprintf("Ordinary child %03d", i), testutil.Cols{"parent_issue_id": shared, "description": "PRIVATE-CHILD-BODY"})
	}
	independent := collaborationBoundaryIssue(t, testWorkspaceID)
	dbfx.Exec(t, `UPDATE issue SET parent_issue_id=$2 WHERE id=$1`, independent, shared)
	for _, user := range []string{testUserID, external} {
		block := read(user)
		items := block["items"].([]any)
		if len(items) != len(ids) || len(block) != 2 {
			t.Fatalf("child references were capped or included independently shared children: %d", len(items))
		}
		if user == external && block["source_workspace_slug"] != nil || user == testUserID && block["source_workspace_slug"] == nil {
			t.Fatal("source-open slug did not follow current membership")
		}
		for i, value := range items {
			item := value.(map[string]any)
			if len(item) != 4 || item["id"] != ids[i] || item["title"] != fmt.Sprintf("Ordinary child %03d", i) || item["can_open"] != (user == testUserID) || item["identifier"] == "" {
				t.Fatalf("child projection/order/authority mismatch: %v", item)
			}
		}
	}
	dbfx.Exec(t, `UPDATE issue SET parent_issue_id=NULL WHERE id=$1`, ids[0])
	if len(read(external)["items"].([]any)) != len(ids)-1 {
		t.Fatal("detached ordinary child remained in source projection")
	}
	dbfx.Exec(t, `UPDATE issue_participant SET revoked_at=now() WHERE issue_id=$1 AND user_id=$2`, shared, external)
	r := withURLParam(newRequest(http.MethodGet, "/api/collaboration/issues/"+shared, nil), "id", shared)
	r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: external, Kind: auth.CredentialJWT}))
	testutil.Call(t, testHandler.GetCollaborationIssue, r).Want(http.StatusNotFound)
}

func TestCollaborationOrdinaryChildWritesKeepParentStateAndEmitCommittedHints(t *testing.T) {
	one, two := collaborationBoundaryIssue(t, testWorkspaceID), collaborationBoundaryIssue(t, testWorkspaceID)
	viewer := dbfx.User(t, "Ordinary source member", "child-writer-"+uuid.NewString()+"@example.test")
	dbfx.Member(t, testWorkspaceID, viewer, "member")
	child := dbfx.Issue(t, "Attachable ordinary child")
	h := *testHandler
	issues := *h.IssueService
	issues.Bus = events.New()
	h.IssueService = &issues
	var hints []string
	issues.Bus.Subscribe(service.CollaborationChildrenChanged, func(event events.Event) {
		payload, ok := event.Payload.(map[string]any)
		if !ok || len(payload) != 1 || event.WorkspaceID != "" || event.ActorID != "" {
			t.Error("child hint escaped its content-free issue scope")
			return
		}
		id, _ := payload["issue_id"].(string)
		hints = append(hints, id)
		// This listener uses a second connection. A hint before commit would see
		// the old child relationship even though the request reports success.
		if len(hints) == 1 {
			var parent *string
			dbfx.QueryRow(t, `SELECT parent_issue_id::text FROM issue WHERE id=$1`, child).Scan(&parent)
			if parent == nil || *parent != one {
				t.Error("child hint was published before commit")
			}
		}
	})
	snapshot := func() string {
		t.Helper()
		var value string
		dbfx.QueryRow(t, `SELECT jsonb_agg(jsonb_build_object('issue',to_jsonb(i),'collaboration',to_jsonb(c),'session',to_jsonb(s),'participants',(SELECT jsonb_agg(to_jsonb(p) ORDER BY p.user_id) FROM issue_participant p WHERE p.issue_id=i.id)) ORDER BY i.id)::text FROM issue i JOIN issue_collaboration c ON c.issue_id=i.id JOIN collaboration_session s ON s.issue_id=i.id WHERE i.id=ANY($1::uuid[])`, []string{one, two}).Scan(&value)
		return value
	}
	before := snapshot()
	request := func(id string, body map[string]any) *http.Request {
		r := withURLParam(newRequest(http.MethodPut, "/api/issues/"+id, body), "id", id)
		r.Header.Set("X-User-ID", viewer)
		return r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: viewer, Kind: auth.CredentialJWT}))
	}
	testutil.Call(t, h.UpdateIssue, request(child, map[string]any{"parent_issue_id": one})).Want(http.StatusOK)
	if len(hints) != 1 || hints[0] != one {
		t.Fatalf("attach omitted parent hint: %v", hints)
	}
	testutil.Call(t, h.UpdateIssue, request(child, map[string]any{"parent_issue_id": two})).Want(http.StatusOK)
	if len(hints) != 3 || hints[1] != one || hints[2] != two {
		t.Fatalf("parent move omitted old/new hints: %v", hints)
	}
	testutil.Call(t, h.UpdateIssue, request(child, map[string]any{"title": "Renamed ordinary child"})).Want(http.StatusOK)
	if len(hints) != 4 || hints[3] != two {
		t.Fatal("rename omitted shared child refresh")
	}
	testutil.Call(t, h.UpdateIssue, request(child, map[string]any{"parent_issue_id": nil})).Want(http.StatusOK)
	if len(hints) != 5 || hints[4] != two {
		t.Fatal("detach omitted old parent refresh")
	}
	foreign := dbfx.Workspace(t, "Unrelated child source", "foreign-children-"+uuid.NewString())
	foreignParent := dbfx.Issue(t, "Foreign parent", testutil.Cols{"workspace_id": foreign})
	testutil.Call(t, h.UpdateIssue, request(child, map[string]any{"parent_issue_id": foreignParent})).Want(http.StatusBadRequest)
	if len(hints) != 5 || snapshot() != before {
		t.Fatal("ordinary child action modified the parent collaboration or notified a refused write")
	}
	// Fixture inserts choose numbers independently of the production allocator.
	dbfx.Exec(t, `UPDATE workspace SET issue_counter=GREATEST(issue_counter,COALESCE((SELECT max(number) FROM issue WHERE workspace_id=$1),0)) WHERE id=$1`, testWorkspaceID)
	create := newRequest(http.MethodPost, "/api/issues", map[string]any{"title": "Created ordinary child " + uuid.NewString(), "parent_issue_id": one})
	create.Header.Set("X-User-ID", viewer)
	create = create.WithContext(auth.WithAuthenticatedPrincipal(create.Context(), auth.AuthenticatedPrincipal{UserID: viewer, Kind: auth.CredentialJWT}))
	created := testutil.Call(t, h.CreateIssue, create).Want(http.StatusCreated).Map()
	createdID, ok := created["id"].(string)
	if !ok || len(hints) != 6 || hints[5] != one {
		t.Fatal("ordinary creation omitted its committed child hint")
	}
	dbfx.Cleanup(t, `DELETE FROM issue WHERE id=$1`, createdID)
	deleteRequest := request(createdID, nil)
	deleteRequest.Method = http.MethodDelete
	testutil.Call(t, h.DeleteIssue, deleteRequest).Want(http.StatusNoContent)
	if len(hints) != 7 || hints[6] != one {
		t.Fatal("ordinary delete omitted its old parent hint")
	}
	first := dbfx.Issue(t, "Batch deleted ordinary child one", testutil.Cols{"parent_issue_id": one})
	second := dbfx.Issue(t, "Batch deleted ordinary child two", testutil.Cols{"parent_issue_id": one})
	batch := newRequest(http.MethodPost, "/api/issues/batch-delete", map[string]any{"issue_ids": []string{first, second}})
	batch.Header.Set("X-User-ID", viewer)
	batch = batch.WithContext(auth.WithAuthenticatedPrincipal(batch.Context(), auth.AuthenticatedPrincipal{UserID: viewer, Kind: auth.CredentialJWT}))
	testutil.Call(t, h.BatchDeleteIssues, batch).Want(http.StatusOK)
	if len(hints) != 8 || hints[7] != one || snapshot() != before {
		t.Fatal("batch deletion duplicated parent hints or changed collaboration state")
	}
	if dbfx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=ANY($1::uuid[])`, []string{one, two}) != 0 || dbfx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=ANY($1::uuid[])`, []string{one, two}) != 0 {
		t.Fatal("child reference edits created workflow history or participant notifications")
	}
}
